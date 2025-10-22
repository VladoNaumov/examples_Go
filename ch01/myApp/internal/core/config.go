package core

// config.go — Загрузка и валидация настроек приложения из переменных окружения.

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"
)

// Config — Настройки приложения, включая таймауты и параметры безопасности.
type Config struct {
	AppName           string        // Имя приложения
	Addr              string        // Адрес HTTP-сервера (например, ":8080")
	Env               string        // Среда выполнения (dev, prod, test)
	CSRFKey           string        // Ключ для CSRF-защиты (криптостойкая строка)
	Secure            bool          // True, если приложение работает в HTTPS-режиме (для secure cookie, HSTS)
	TLSOffloaded      bool          // True, если TLS завершается на прокси (Nginx/LB)
	CertFile          string        // Путь к TLS-сертификату (если TLS не offloaded)
	KeyFile           string        // Путь к TLS-ключу (если TLS не offloaded)
	ShutdownTimeout   time.Duration // Таймаут для корректного завершения работы сервера
	ReadHeaderTimeout time.Duration // Таймаут чтения заголовков HTTP
	ReadTimeout       time.Duration // Таймаут чтения всего тела HTTP-запроса
	WriteTimeout      time.Duration // Таймаут записи HTTP-ответа
	IdleTimeout       time.Duration // Таймаут простоя соединения
	RequestTimeout    time.Duration // Общий таймаут на обработку запроса в middleware
}

// fatalConfigError — централизованно логирует ошибку конфигурации и завершает работу.
// Используется, когда невозможно продолжить работу из-за критической ошибки в Prod.
func fatalConfigError(msg string, fields map[string]interface{}) {
	// Используем LogError, чтобы записать ошибку в файл (если logger инициализирован)
	LogError("Критическая ошибка конфигурации", fields)

	// Дополнительный вывод в stderr, так как LogError может быть асинхронным
	_, _ = fmt.Fprintf(os.Stderr, "FATAL CONFIG ERROR: %s\n", msg)

	// Завершение работы
	os.Exit(1)
}

// Load загружает конфигурацию из ENV с дефолтами и проводит валидацию для Prod.
func Load() Config {
	cfg := Config{
		AppName:           getEnv("APP_NAME", "myApp"),
		Addr:              getEnv("HTTP_ADDR", ":8080"),
		Env:               getEnv("APP_ENV", "dev"),
		CSRFKey:           getEnv("CSRF_KEY", generateRandomKey()), // Криптостойкий дефолт
		Secure:            getEnvBool("SECURE", false),
		TLSOffloaded:      getEnvBool("TLS_OFFLOADED", false), // если true — TLS у nginx
		CertFile:          getEnv("TLS_CERT_FILE", ""),
		KeyFile:           getEnv("TLS_KEY_FILE", ""),
		ShutdownTimeout:   getEnvDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
		ReadHeaderTimeout: getEnvDuration("READ_HEADER_TIMEOUT", 5*time.Second),
		ReadTimeout:       getEnvDuration("READ_TIMEOUT", 10*time.Second),
		WriteTimeout:      getEnvDuration("WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:       getEnvDuration("IDLE_TIMEOUT", 60*time.Second),
		RequestTimeout:    getEnvDuration("REQUEST_TIMEOUT", 15*time.Second),
	}

	// Валидация для продакшена — ключевой этап безопасности и отказоустойчивости
	if strings.ToLower(cfg.Env) == "prod" {

		// 1. Проверка силы CSRF-ключа (минимум 32 байта)
		if !isKeyStrong(cfg.CSRFKey, 32) {
			fatalConfigError(
				"Недостаточная длина CSRF_KEY в продакшене. Требуется минимум 32 байта.",
				map[string]interface{}{"key": "CSRF_KEY", "provided_length": len(cfg.CSRFKey)},
			)
		}

		// 2. Проверка адреса
		if cfg.Addr == "" {
			fatalConfigError("Отсутствует HTTP_ADDR в продакшене.", map[string]interface{}{"key": "HTTP_ADDR"})
		}

		// 3. Требование HTTPS-режима (Secure=true)
		// Гарантируем, что приложение ставит безопасные куки и HSTS (если он включен).
		if !cfg.Secure {
			fatalConfigError(
				"SECURE должен быть true в продакшене. Приложение должно работать в HTTPS-режиме (даже при offload на прокси).",
				map[string]interface{}{"key": "SECURE", "tip": "Установите SECURE=true"},
			)
		}

		// 4. Проверка файлов TLS (если TLS не offloaded)
		if !cfg.TLSOffloaded && (cfg.CertFile == "" || cfg.KeyFile == "") {
			fatalConfigError(
				"TLS_CERT_FILE / TLS_KEY_FILE отсутствуют, а TLS не offloaded. Требуются файлы сертификата/ключа.",
				map[string]interface{}{"keys_missing": []string{"TLS_CERT_FILE", "TLS_KEY_FILE"}},
			)
		}
	}

	return cfg
}

// getEnv — Извлекает строку из ENV, убирает пробелы, или возвращает дефолт.
func getEnv(key, def string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		return val
	}
	return def
}

// getEnvBool — Извлекает bool из ENV. Поддерживает "true", "1", "yes", "on".
func getEnvBool(key string, def bool) bool {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return def
	}
	v := strings.ToLower(val)
	return v == "true" || v == "1" || v == "yes" || v == "on"
}

// getEnvDuration — Извлекает time.Duration из ENV. Поддерживает форматы Go ("30s") или просто число (интерпретируется как секунды).
func getEnvDuration(key string, def time.Duration) time.Duration {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return def
	}

	// 1. Попытка распарсить как стандартную длительность Go ("10s", "1m")
	if d, err := time.ParseDuration(val); err == nil {
		return d
	}

	// 2. Если не стандарт, пытаемся интерпретировать как число секунд
	if secs, err := time.ParseDuration(val + "s"); err == nil {
		return secs
	}

	// 3. Ошибка формата, возвращаем дефолт и логируем
	LogError("Неверный формат длительности. Используется дефолт.", map[string]interface{}{"key": key, "value": val, "default": def.String()})
	return def
}

// generateRandomKey — Генерирует криптостойкий ключ (32 байта) и кодирует его в Base64.
// Используется как дефолтное значение для CSRFKey.
func generateRandomKey() string {
	b := make([]byte, 32)
	// Читаем криптостойкие случайные байты
	if _, err := rand.Read(b); err != nil {
		LogError("Ошибка генерации CSRF-ключа", map[string]interface{}{"error": err.Error()})
		// Критическое исключение: невозможно сгенерировать безопасный ключ
		panic("невозможно сгенерировать криптостойкий ключ")
	}
	return base64.StdEncoding.EncodeToString(b)
}

// isKeyStrong проверяет, что ключ содержит минимум minBytes байт.
// Учитывает, что ключ может быть закодирован в Base64.
func isKeyStrong(key string, minBytes int) bool {
	if key == "" {
		return false
	}

	// Попытка декодировать как Base64 (если это ключ, сгенерированный нашей функцией)
	if decoded, err := base64.StdEncoding.DecodeString(key); err == nil {
		return len(decoded) >= minBytes
	}

	// Иначе, проверяем raw-байтовую длину строки
	return len([]byte(key)) >= minBytes
}
