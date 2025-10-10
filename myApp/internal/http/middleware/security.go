package middleware

// security.go

import (
	"net/http"
	"strings"
)

// SecureHeaders добавляет безопасные HTTP-заголовки для защиты от XSS, clickjacking и других атак.
func SecureHeaders(nonce string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// CSP: ограничивает источники ресурсов, минимизирует XSS (OWASP A03).
			w.Header().Set("Content-Security-Policy", ""+
				"default-src 'self'; "+
				"img-src 'self' data:; "+
				"style-src 'self' https://cdn.jsdelivr.net 'nonce-"+nonce+"'; "+
				"script-src 'self' https://cdn.jsdelivr.net; "+
				"font-src 'self' https://cdn.jsdelivr.net data:; "+
				"connect-src 'self' https://cdn.jsdelivr.net; "+
				"form-action 'self'; "+
				"frame-ancestors 'none'")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "no-referrer-when-downgrade")
			w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
			w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")

			// Проверка HTTP Parameter Pollution (OWASP A03).
			for _, values := range r.URL.Query() {
				if len(values) > 1 {
					http.Error(w, "Multiple values for query parameter detected", http.StatusBadRequest)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// HSTS включает HTTPS-only в prod (OWASP A02).
func HSTS(isProduction bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isProduction && r.TLS != nil {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CacheStatic задаёт кэширование для статики (OWASP A05).
func CacheStatic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".css") || strings.HasSuffix(r.URL.Path, ".js") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}
		w.Header().Set("Vary", "Accept-Encoding")
		next.ServeHTTP(w, r)
	})
}
