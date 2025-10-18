package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ==== Константы (JSON API + Nginx) ====

const (
	// Сетевые настройки (Nginx проксирует на 127.0.0.1:8080)
	APIAddr           = "127.0.0.1:8080" // Только localhost для безопасности
	ReadHeaderTimeout = 5 * time.Second
	ReadBodyTimeout   = 30 * time.Second // Больше для JSON
	WriteTimeout      = 60 * time.Second // JSON может быть медленнее
	IdleTimeout       = 5 * time.Minute  // Дольше для keep-alive
	MaxHeaderBytes    = 1 << 20          // 1MB заголовков
	MaxBodyBytes      = 10 << 20         // 10MB JSON

	// Безопасность (Nginx обрабатывает Host validation)
	AllowedOrigins       = "https://example.com,https://app.example.com" // Ваши фронтенды
	RateLimitMaxRequests = 200                                           // Больше для API
	RateLimitWindow      = 1 * time.Minute
	SessionTokenLength   = 32
	SessionMaxAge        = 24 * 3600 // 24 часа
	MaxUploadFileMB      = 10

	// JSON API настройки
	JSONIndent     = false          // false = компактный JSON
	CSRFHeaderName = "X-CSRF-Token" // Для API клиентов
)

// ==== Конфигурация ====

type Config struct {
	Addr              string
	AllowedOrigins    []string
	MaxHeaderBytes    int
	MaxBodyBytes      int64
	ReadHeaderTimeout time.Duration
	ReadBodyTimeout   time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	RateLimitMax      int
	RateLimitWindow   time.Duration
}

func LoadConfig() Config {
	return Config{
		Addr:              APIAddr,
		AllowedOrigins:    splitCSV(AllowedOrigins),
		MaxHeaderBytes:    MaxHeaderBytes,
		MaxBodyBytes:      MaxBodyBytes,
		ReadHeaderTimeout: ReadHeaderTimeout,
		ReadBodyTimeout:   ReadBodyTimeout,
		WriteTimeout:      WriteTimeout,
		IdleTimeout:       IdleTimeout,
		RateLimitMax:      RateLimitMaxRequests,
		RateLimitWindow:   RateLimitWindow,
	}
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ==== Rate Limiter ====

type rateLimiter struct {
	requests map[string][]time.Time
	mu       sync.RWMutex
	max      int
	window   time.Duration
}

func newRateLimiter(max int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		requests: make(map[string][]time.Time),
		max:      max,
		window:   window,
	}
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	reqs := rl.requests[ip]

	// Очистка старых записей
	reqs = filterRecent(reqs, now, rl.window)

	if len(reqs) >= rl.max {
		return false
	}

	reqs = append(reqs, now)
	rl.requests[ip] = reqs
	return true
}

func filterRecent(times []time.Time, now time.Time, window time.Duration) []time.Time {
	var recent []time.Time
	cutoff := now.Add(-window)
	for _, t := range times {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	return recent
}

// ==== JSON Response Helper ====

type jsonResponse struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data,omitempty"`
	Error  string      `json:"error,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	resp := jsonResponse{Status: "ok", Data: data}
	if status >= 400 {
		resp.Status = "error"
		resp.Error = fmt.Sprintf("%d: %v", status, data)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "")
	if JSONIndent {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(resp)
}

// ==== Утилиты ====

func clientIP(r *http.Request) string {
	// Nginx передает X-Real-IP или X-Forwarded-For
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// Берем первый IP (клиент)
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	// Fallback
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func validateOrigin(allowed []string, originStr string) bool {
	u, err := url.Parse(originStr)
	if err != nil {
		return false
	}

	for _, allowedStr := range allowed {
		au, err := url.Parse(allowedStr)
		if err != nil {
			continue
		}
		if u.Scheme == au.Scheme && u.Host == au.Host {
			return true
		}
	}
	return false
}

func isStateChanging(method string) bool {
	return method == http.MethodPost || method == http.MethodPut ||
		method == http.MethodPatch || method == http.MethodDelete
}

// ==== Middleware ====

type middleware func(http.Handler) http.Handler

func secureHeaders() middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// API-focused security headers
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			// Strict CSP for API
			w.Header().Set("Content-Security-Policy",
				"default-src 'none'; connect-src 'self'; frame-ancestors 'none'")
			next.ServeHTTP(w, r)
		})
	}
}

func limitBody(max int64) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, max)
			next.ServeHTTP(w, r)
		})
	}
}

func corsStrict(allowedOrigins []string) middleware {
	origins := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		origins[o] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				if _, ok := origins[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", "Origin")
					w.Header().Set("Access-Control-Allow-Methods",
						"GET,POST,PUT,PATCH,DELETE,OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers",
						"Content-Type,Authorization,"+CSRFHeaderName)
					w.Header().Set("Access-Control-Allow-Credentials", "true")

					if r.Method == http.MethodOptions {
						w.WriteHeader(http.StatusNoContent)
						return
					}
				} else if r.Method == http.MethodOptions {
					http.Error(w, "CORS forbidden", http.StatusForbidden)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func csrfGuard(allowedOrigins []string) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isStateChanging(r.Method) {
				next.ServeHTTP(w, r)
				return
			}

			// API CSRF: проверяем Origin + CSRF токен
			origin := r.Header.Get("Origin")
			csrfToken := r.Header.Get(CSRFHeaderName)

			if origin == "" || !validateOrigin(allowedOrigins, origin) {
				writeJSON(w, http.StatusForbidden, "invalid origin")
				return
			}

			if csrfToken == "" {
				writeJSON(w, http.StatusForbidden, "CSRF token required")
				return
			}

			// В реальности: валидация токена против сессии
			// sessionToken := getSessionToken(r)
			// if !validateCSRF(sessionToken, csrfToken) { ... }

			next.ServeHTTP(w, r)
		})
	}
}

func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic: %v", rec)
				writeJSON(w, http.StatusInternalServerError, "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		ip := clientIP(r)

		next.ServeHTTP(rw, r)

		log.Printf("%s %s %d %s ip=%s size=%d dur=%v",
			r.Method, r.URL.Path, rw.status, r.Proto, ip,
			r.ContentLength, time.Since(start))
	})
}

func rateLimit(rl *rateLimiter) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !rl.allow(clientIP(r)) {
				writeJSON(w, http.StatusTooManyRequests, "rate limited")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ==== Хэндлеры ====

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": "1.0"})
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	// Парсим JSON credentials
	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		writeJSON(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	// В реальности: валидация credentials
	token, err := randomToken(SessionTokenLength / 2)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "token generation failed")
		return
	}

	// CSRF токен для клиента
	csrfToken, _ := randomToken(16)

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		MaxAge:   SessionMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":     "ok",
		"csrf_token": csrfToken,
		// В реальности: JWT или session ID
	})
}

func uploadHandler(maxMB int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// CSRF уже проверен middleware'ом

		if err := r.ParseMultipartForm(maxMB << 20); err != nil {
			writeJSON(w, http.StatusBadRequest, "invalid multipart form")
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, "file required")
			return
		}
		defer file.Close()

		// Безопасная валидация
		name := filepath.Base(header.Filename)
		if name == "." || strings.Contains(name, "..") || strings.ContainsAny(name, "/\\") {
			writeJSON(w, http.StatusBadRequest, "invalid filename")
			return
		}

		ext := strings.ToLower(filepath.Ext(name))
		allowedExts := map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true}
		if !allowedExts[ext] {
			writeJSON(w, http.StatusUnsupportedMediaType, "unsupported file type")
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":   "uploaded",
			"filename": name,
			"size":     header.Size,
		})
	}
}

// ==== Сборка ====

func chain(h http.Handler, m ...middleware) http.Handler {
	for i := len(m) - 1; i >= 0; i-- {
		h = m[i](h)
	}
	return h
}

// ==== Main ====

func main() {
	cfg := LoadConfig()
	rl := newRateLimiter(cfg.RateLimitMax, cfg.RateLimitWindow)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler)
	mux.HandleFunc("/api/login", loginHandler)
	mux.Handle("/api/upload", uploadHandler(MaxUploadFileMB))

	// API-only middleware stack
	handler := chain(
		mux,
		requestLogger,
		recoverer,
		rateLimit(rl),
		secureHeaders(),
		csrfGuard(cfg.AllowedOrigins),
		corsStrict(cfg.AllowedOrigins),
		limitBody(cfg.MaxBodyBytes),
	)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
	}

	// Graceful shutdown
	idleConnsClosed := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint

		log.Println("shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
		close(idleConnsClosed)
	}()

	log.Printf("JSON API starting on %s", cfg.Addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
	<-idleConnsClosed
}
