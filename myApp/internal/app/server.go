package app

import (
	"net/http"
	"time"

	"context"
	"myApp/internal/core"
)

// Server создаёт http.Server с таймаутами (OWASP A05).
func Server(cfg core.Config, h http.Handler) (*http.Server, error) {
	return &http.Server{
		Addr:              cfg.Addr,
		Handler:           h,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}, nil
}

// Shutdown завершает сервер с таймаутом (OWASP A05).
func Shutdown(srv *http.Server, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return srv.Shutdown(ctx)
}
