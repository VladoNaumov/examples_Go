package middleware

// security.go
import (
	"net/http"

	"myApp/internal/core"
)

// SecureHeaders добавляет заголовки безопасности (OWASP A03, A05).
// Nonce берётся из r.Context() -> core.CtxNonce и подставляется в CSP.
func SecureHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nonce, _ := r.Context().Value(core.CtxNonce).(string)

			// CSP с nonce для инлайн-скриптов/стилей при необходимости.
			// Если у тебя нет инлайн-STYLE — можно убрать 'nonce-...' из style-src.
			csp := "" +
				"default-src 'self'; " +
				"img-src 'self' data:; " +
				"style-src 'self' https://cdn.jsdelivr.net 'unsafe-inline' 'nonce-" + nonce + "'; " +
				"script-src 'self' https://cdn.jsdelivr.net 'nonce-" + nonce + "'; " +
				"font-src 'self' https://cdn.jsdelivr.net data:; " +
				"connect-src 'self' https://cdn.jsdelivr.net; " +
				"form-action 'self'; " +
				"frame-ancestors 'none'; " +
				"base-uri 'self'"

			w.Header().Set("Content-Security-Policy", csp)
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
			w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")

			// Лёгкая защита от Query Parameter Pollution (только для GET querystring).
			for _, vv := range r.URL.Query() {
				if len(vv) > 1 {
					http.Error(w, "Multiple values for query parameter detected", http.StatusBadRequest)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// HSTS включает HTTPS-only в prod (OWASP A02).
// Заголовок безвредно отдать всегда в prod — HTTP-клиент его проигнорирует на не-HTTPS.
func HSTS(isProduction bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isProduction {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
			}
			next.ServeHTTP(w, r)
		})
	}
}
