package middleware

//proxy.go
import (
	"net"
	"net/http"
	"strings"
)

// TrustedProxy проверяет, что запросы поступают от доверенных прокси, и устанавливает реальный IP и схему (OWASP A05: Security Misconfiguration)
func TrustedProxy(trustedIPs []string) func(http.Handler) http.Handler {
	trusted := make(map[string]struct{})
	for _, ip := range trustedIPs {
		trusted[ip] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Извлекает IP клиента из RemoteAddr
			clientIP, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil || clientIP == "" {
				http.Error(w, "Неверный адрес клиента", http.StatusBadRequest)
				return
			}

			// Проверяет, что IP входит в список доверенных
			if _, ok := trusted[clientIP]; !ok {
				http.Error(w, "Недоверенный прокси", http.StatusForbidden)
				return
			}

			// Устанавливает реальный IP из заголовка X-Forwarded-For
			if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
				// Разделяет список IP и берёт первый валидный
				ips := strings.Split(forwarded, ",")
				for _, ip := range ips {
					ip = strings.TrimSpace(ip)
					if parsedIP := net.ParseIP(ip); parsedIP != nil {
						r.RemoteAddr = ip
						break
					}
				}
			}

			// Устанавливает схему HTTPS, если указан в X-Forwarded-Proto
			if proto := r.Header.Get("X-Forwarded-Proto"); proto == "https" {
				r.URL.Scheme = "https"
			}

			next.ServeHTTP(w, r)
		})
	}
}
