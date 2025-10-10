package app

//server.go
import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"myApp/internal/core"
)

// Server создаёт http.Server с таймаутами и TLS (OWASP A02, A05).
func Server(cfg core.Config, handler http.Handler) (*http.Server, error) {
	if cfg.Addr == "" {
		return nil, fmt.Errorf("server address cannot be empty")
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	if cfg.Secure {
		srv.TLSConfig = &tls.Config{
			MinVersion:               tls.VersionTLS12,
			PreferServerCipherSuites: true,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			},
		}
	}

	return srv, nil
}

// Shutdown выполняет graceful shutdown (OWASP A09).
func Shutdown(srv *http.Server, timeout time.Duration) error {
	defer core.Close()

	done := make(chan error, 1)
	go func() {
		done <- srv.Shutdown(nil)
	}()

	select {
	case err := <-done:
		if err != nil {
			core.LogError("Server shutdown failed", map[string]interface{}{"error": err.Error()})
			return err
		}
	case <-time.After(timeout):
		core.LogError("Server shutdown timed out", nil)
		return fmt.Errorf("server shutdown timed out after %s", timeout)
	}
	return nil
}
