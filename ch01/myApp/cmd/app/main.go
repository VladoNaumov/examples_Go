package main

// main.go — точка входа приложения myApp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"myApp/internal/app"
	"myApp/internal/core"
	"myApp/internal/storage"

	"golang.org/x/crypto/pbkdf2"
)

func main() {
	// Загружаем конфиг (из .env, переменных окружения или файла)
	cfg := core.Load()

	// Логируем старт с параметрами окружения
	core.LogInfo("Приложение запущено", map[string]interface{}{
		"env":    cfg.Env,    // режим: dev / prod
		"addr":   cfg.Addr,   // адрес HTTP-сервера
		"secure": cfg.Secure, // HTTPS включён или нет
		"app":    cfg.AppName,
	})

	// Инициализируем ежедневный лог-файл (по дате)
	core.InitDailyLog()

	// Подключаем базу данных (sqlx.DB)
	db, err := storage.NewDB()
	if err != nil {
		core.LogError("Ошибка БД", map[string]interface{}{"error": err})
		os.Exit(1)
	}

	// Запускаем миграции, если есть (обновление структуры БД)
	migrations := storage.NewMigrations(db)
	if err := migrations.RunMigrations(); err != nil {
		core.LogError("Ошибка миграций", map[string]interface{}{"error": err})
		os.Exit(1)
	}

	// Генерируем или производим derivation CSRF-ключа (32 байта)
	// Используется для защиты форм и сессий
	csrfKey := deriveSecureKey(cfg.CSRFKey)

	// Инициализируем приложение (Gin, middleware, маршруты)
	handler, err := app.New(cfg, db, csrfKey)
	if err != nil {
		core.LogError("Ошибка app.New", map[string]interface{}{"error": err})
		os.Exit(1)
	}

	// Создаём HTTP-сервер с таймаутами
	srv := newHTTPServer(cfg, handler)

	// Создаём контекст, который будет отменён при сигнале SIGINT/SIGTERM
	// (нужно для graceful shutdown)
	sigs, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Запускаем сервер в отдельной горутине
	go runServer(srv, cfg)

	// Ждём сигнал завершения (Ctrl+C или systemd stop)
	<-sigs.Done()
	core.LogInfo("Завершение...", nil)

	// Плавно останавливаем сервер
	if err := srv.Shutdown(context.Background()); err != nil {
		core.LogError("Ошибка shutdown", map[string]interface{}{"error": err})
	}

	// Закрываем соединение с БД
	_ = storage.Close(db)

	// Закрываем логи, если нужно
	core.Close()
}

// newHTTPServer — создаёт http.Server с параметрами из конфига
func newHTTPServer(cfg core.Config, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.Addr,              // адрес (например ":8080")
		Handler:           h,                     // обработчик (Gin engine)
		ReadHeaderTimeout: cfg.ReadHeaderTimeout, // таймаут заголовков
		ReadTimeout:       cfg.ReadTimeout,       // общий таймаут чтения
		WriteTimeout:      cfg.WriteTimeout,      // таймаут записи
		IdleTimeout:       cfg.IdleTimeout,       // таймаут keep-alive
	}
}

// runServer — запускает сервер и логирует падения
func runServer(srv *http.Server, cfg core.Config) {
	core.LogInfo("Сервер запущен", map[string]interface{}{"addr": cfg.Addr})

	// Запускаем HTTP-сервер
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		// Если ошибка не "сервер закрыт" — это краш
		core.LogError("Сервер упал", map[string]interface{}{"error": err})
		os.Exit(1)
	}
}

// deriveSecureKey — генерирует 32-байтовый криптографически стойкий ключ для CSRF, если secret пустой — создаёт новый.
func deriveSecureKey(secret string) []byte {
	if len(secret) == 0 {
		// Если в конфиге нет ключа — генерируем случайный
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			// Сюда почти никогда не попадём (rand.Read редко падает)
			panic("unable to generate random bytes: " + err.Error())
		}
		return b
	}

	// Если ключ задан, "растягиваем" его через PBKDF2
	// — безопасный способ получить ключ фиксированной длины
	salt := []byte("myapp-session-salt")
	return pbkdf2.Key([]byte(secret), salt, 4096, 32, sha256.New)
}
