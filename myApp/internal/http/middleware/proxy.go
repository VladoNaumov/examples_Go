package middleware

import (
	"net"
	"net/http"
)

// TrustedProxy обрабатывает X-Forwarded-For и X-Forwarded-Proto (OWASP A05).
func TrustedProxy(trustedIPs []string) func(http.Handler) http.Handler {
	trusted := make(map[string]struct{})
	for _, ip := range trustedIPs {
		trusted[ip] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Проверяем, что запрос от доверенного прокси.
			clientIP, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil || clientIP == "" {
				http.Error(w, "Invalid client address", http.StatusBadRequest)
				return
			}
			if _, ok := trusted[clientIP]; !ok {
				http.Error(w, "Untrusted proxy", http.StatusForbidden)
				return
			}

			// Устанавливаем реальный IP из X-Forwarded-For.
			if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
				ips := net.ParseIP(forwarded)
				if ips != nil {
					r.RemoteAddr = forwarded
				}
			}

			// Проверяем HTTPS через X-Forwarded-Proto.
			if proto := r.Header.Get("X-Forwarded-Proto"); proto == "https" {
				r.URL.Scheme = "https"
			}

			next.ServeHTTP(w, r)
		})
	}
}
