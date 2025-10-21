package core

// config.go

import (
	"crypto/rand"
	"encoding/base64"
	"os"
	"strings"
	"time"
)

// Config — настройки приложения
type Config struct {
	AppName           string        // Имя приложения
	Addr              string        // Адрес HTTP-сервера (например, ":8080")
	Env               string        // Среда выполнения (dev, prod)
	CSRFKey           string        // Ключ для CSRF-защиты
	Secure            bool          // Включает HTTPS и связанные настройки безопасности
	CertFile          string        // Путь к TLS-сертификату
	KeyFile           string        // Путь к TLS-ключу
	ShutdownTimeout   time.Duration // Таймаут для graceful shutdown
	ReadHeaderTimeout time.Duration // Таймаут чтения заголовков HTTP-запроса
	ReadTimeout       time.Duration // Таймаут чтения HTTP-запроса
	WriteTimeout      time.Duration // Таймаут записи HTTP-ответа
	IdleTimeout       time.Duration // Таймаут простоя соединения
	RequestTimeout    time.Duration // Таймаут обработки запроса в middleware
}

// Load загружает конфигурацию из ENV с дефолтами
func Load() Config {
	cfg := Config{
		AppName:           getEnv("APP_NAME", "myApp"),
		Addr:              getEnv("HTTP_ADDR", ":8080"),
		Env:               getEnv("APP_ENV", "dev"),
		CSRFKey:           getEnv("CSRF_KEY", generateRandomKey()),
		Secure:            getEnvBool("SECURE", false),
		CertFile:          getEnv("TLS_CERT_FILE", ""),
		KeyFile:           getEnv("TLS_KEY_FILE", ""),
		ShutdownTimeout:   getEnvDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
		ReadHeaderTimeout: getEnvDuration("READ_HEADER_TIMEOUT", 5*time.Second),
		ReadTimeout:       getEnvDuration("READ_TIMEOUT", 10*time.Second),
		WriteTimeout:      getEnvDuration("WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:       getEnvDuration("IDLE_TIMEOUT", 60*time.Second),
		RequestTimeout:    getEnvDuration("REQUEST_TIMEOUT", 15*time.Second),
	}

	// Валидация для prod
	if cfg.Env == "prod" {
		if len(cfg.CSRFKey) < 32 {
			LogError("Недостаточная длина CSRF_KEY в продакшене", map[string]interface{}{"length": len(cfg.CSRFKey)})
			os.Exit(1)
		}
		if cfg.Secure && (cfg.CertFile == "" || cfg.KeyFile == "") {
			LogError("Отсутствует TLS_CERT_FILE или TLS_KEY_FILE в продакшене", nil)
			os.Exit(1)
		}
		if cfg.Addr == "" {
			LogError("Отсутствует HTTP_ADDR в продакшене", nil)
			os.Exit(1)
		}
		if !cfg.Secure {
			LogError("SECURE должен быть true в продакшене для использования HTTPS", nil)
			os.Exit(1)
		}
	}

	return cfg
}

// getEnv — строка из ENV или дефолт
func getEnv(key, def string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		return val
	}
	return def
}

// getEnvBool — bool из ENV (поддержка true/1)
func getEnvBool(key string, def bool) bool {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return def
	}
	return val == "true" || val == "1" || val == "TRUE"
}

// getEnvDuration — time.Duration из ENV
func getEnvDuration(key string, def time.Duration) time.Duration {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return def
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		LogError("Неверный формат длительности", map[string]interface{}{"key": key, "value": val, "error": err.Error()})
		return def
	}
	return d
}

// generateRandomKey — криптостойкий ключ 32 байта → base64
func generateRandomKey() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		LogError("Ошибка генерации CSRF-ключа", map[string]interface{}{"error": err.Error()})
		panic("невозможно сгенерировать ключ")
	}
	return base64.StdEncoding.EncodeToString(b)
}
