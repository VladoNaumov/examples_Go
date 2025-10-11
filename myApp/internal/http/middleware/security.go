package middleware

import (
	"net/http"
)

// SecureHeaders добавляет заголовки безопасности (OWASP A03, A05).
func SecureHeaders(nonce string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// CSP с nonce.
			w.Header().Set("Content-Security-Policy",
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

			// Защита от Parameter Pollution (OWASP A03).
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
			if isProduction && r.URL.Scheme == "https" {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
			}
			next.ServeHTTP(w, r)
		})
	}
}
