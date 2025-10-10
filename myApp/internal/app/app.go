package app

// app.go
import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"time"

	"myApp/internal/core"
	"myApp/internal/http/handler"
	mw "myApp/internal/http/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/csrf"
	"golang.org/x/time/rate"
)

// New собирает приложение (OWASP A05).
func New(cfg core.Config, csrfKey []byte) http.Handler {
	r := chi.NewRouter()

	// Генерация nonce для CSP (OWASP A03).
	nonce, err := generateNonce()
	if err != nil {
		core.LogError("Failed to generate nonce", map[string]interface{}{"error": err.Error()})
		nonce = ""
	}

	// Передача nonce в handlers через context.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), "nonce", nonce)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})

	// Middleware (OWASP A05, A09).
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(15 * time.Second))
	r.Use(mw.SecureHeaders(nonce))
	r.Use(rateLimit(100, 1*time.Second))

	if cfg.Secure {
		r.Use(mw.HSTS(cfg.Env == "prod"))
	}

	r.Use(csrf.Protect(
		csrfKey,
		csrf.Secure(cfg.Secure),
		csrf.SameSite(csrf.SameSiteLaxMode),
		csrf.HttpOnly(true),
		csrf.Path("/"),
	))
	if !cfg.Secure {
		core.LogError("CSRF running without HTTPS in non-prod", nil)
	}

	// Статические файлы (OWASP A05).
	static := http.FileServer(http.Dir("web/assets"))
	if cfg.Env == "prod" {
		static = mw.CacheStatic(static)
	}
	r.Handle("/assets/*", http.StripPrefix("/assets/", static))

	// Маршруты.
	r.Get("/", handler.Home)
	r.Get("/about", handler.About)
	r.Get("/form", handler.FormIndex)
	r.Post("/form", handler.FormSubmit)
	r.Get("/healthz", handler.Health)
	r.NotFound(handler.NotFound)

	return r
}

// generateNonce создаёт nonce для CSP (OWASP A03).
func generateNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// rateLimit ограничивает запросы (OWASP A05).
func rateLimit(rps float64, burst time.Duration) func(http.Handler) http.Handler {
	limiter := rate.NewLimiter(rate.Limit(rps), int(rps))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.Allow() {
				core.Fail(w, r, core.BadRequest("too many requests", nil))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
