package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"myApp/internal/app"
	"myApp/internal/core"
)

func main() {
	cfg := core.Load()
	core.InitDailyLog()
	defer core.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				next := time.Now().Add(24 * time.Hour).Truncate(24 * time.Hour)
				time.Sleep(time.Until(next))
				core.InitDailyLog()
			}
		}
	}()

	// Проверки для prod (OWASP A05).
	if cfg.Env == "prod" {
		if len(cfg.CSRFKey) < 32 {
			core.LogError("Invalid CSRF_KEY in production", map[string]interface{}{"length": len(cfg.CSRFKey)})
			os.Exit(1)
		}
		if cfg.Addr == "" {
			core.LogError("Invalid HTTP_ADDR in production", nil)
			os.Exit(1)
		}
	}

	handler, err := app.New(cfg, derive32(cfg.CSRFKey))
	if err != nil {
		core.LogError("Failed to initialize app", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}

	srv, err := app.Server(cfg, handler)
	if err != nil {
		core.LogError("Failed to create server", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}

	sigs, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("INFO: http: listening addr=%s env=%s app=%s", cfg.Addr, cfg.Env, cfg.AppName)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			core.LogError("Server error", map[string]interface{}{"error": err.Error()})
			os.Exit(1)
		}
	}()

	<-sigs.Done()
	log.Println("INFO: http: shutdown started")
	if err := app.Shutdown(srv, cfg.ShutdownTimeout); err != nil {
		core.LogError("Shutdown error", map[string]interface{}{"error": err.Error()})
	} else {
		log.Println("INFO: http: shutdown complete")
	}
}

// derive32 создаёт 32-байтовый ключ для CSRF (OWASP A07).
func derive32(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}
