package middleware

// security.go

import (
	"net/http"

	"myApp/internal/core"
)

// SecureHeaders добавляет заголовки безопасности, включая CSP с nonce из контекста (OWASP A03: Injection, A05: Security Misconfiguration)
func SecureHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Получает nonce из контекста запроса
			nonce, ok := r.Context().Value(core.CtxNonce).(string)
			if !ok || nonce == "" {
				core.Fail(w, r, core.Internal("Nonce не найден в контексте", nil))
				return
			}

			// Настраивает Content Security Policy с поддержкой nonce
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

			// Устанавливает заголовки безопасности
			w.Header().Set("Content-Security-Policy", csp)
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
			w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")

			// Проверяет на Query Parameter Pollution для GET-запросов
			for _, vv := range r.URL.Query() {
				if len(vv) > 1 {
					http.Error(w, "Обнаружено несколько значений для параметра запроса", http.StatusBadRequest)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// HSTS включает Strict-Transport-Security для продакшен-среды (OWASP A02: Cryptographic Failures)
func HSTS(isProduction bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Добавляет HSTS-заголовок в продакшене
			if isProduction {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
			}
			next.ServeHTTP(w, r)
		})
	}
}
