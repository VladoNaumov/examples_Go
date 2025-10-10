package core

//config.go

import (
	"crypto/rand"
	"encoding/base64"
	"os"
	"strings"
	"time"
)

// Config — настройки приложения (OWASP A05, A02).
type Config struct {
	AppName           string
	Addr              string
	Env               string
	CSRFKey           string
	Secure            bool
	CertFile          string
	KeyFile           string
	ShutdownTimeout   time.Duration
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

// Load загружает конфигурацию (OWASP A05).
func Load() Config {
	cfg := Config{
		AppName:           getEnv("APP_NAME", "myApp"),
		Addr:              getEnv("HTTP_ADDR", ":8080"),
		Env:               getEnv("APP_ENV", "dev"),
		CSRFKey:           getEnv("CSRF_KEY", generateRandomKey()),
		Secure:            getEnv("SECURE", "") == "true",
		CertFile:          getEnv("TLS_CERT_FILE", ""),
		KeyFile:           getEnv("TLS_KEY_FILE", ""),
		ShutdownTimeout:   getEnvDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
		ReadHeaderTimeout: getEnvDuration("READ_HEADER_TIMEOUT", 5*time.Second),
		ReadTimeout:       getEnvDuration("READ_TIMEOUT", 10*time.Second),
		WriteTimeout:      getEnvDuration("WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:       getEnvDuration("IDLE_TIMEOUT", 60*time.Second),
	}

	if cfg.Env == "prod" {
		if len(cfg.CSRFKey) < 32 {
			LogError("Invalid CSRF_KEY in production", map[string]interface{}{"length": len(cfg.CSRFKey)})
			os.Exit(1)
		}
		if cfg.Secure && (cfg.CertFile == "" || cfg.KeyFile == "") {
			LogError("Missing TLS_CERT_FILE or TLS_KEY_FILE in production", nil)
			os.Exit(1)
		}
		if cfg.Addr == "" {
			LogError("Invalid HTTP_ADDR in production", nil)
			os.Exit(1)
		}
	}

	return cfg
}

func getEnv(key, def string) string {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return def
	}
	return val
}

func getEnvDuration(key string, def time.Duration) time.Duration {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return def
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		LogError("Invalid duration format", map[string]interface{}{"key": key, "value": val, "error": err.Error()})
		return def
	}
	return d
}

func generateRandomKey() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		LogError("Failed to generate CSRF key", map[string]interface{}{"error": err.Error()})
		return "fallback-key-please-change"
	}
	return base64.StdEncoding.EncodeToString(b)
}
