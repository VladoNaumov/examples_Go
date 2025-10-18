package app

// app.go
import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"

	"myApp/internal/core"
	"myApp/internal/http/handler"
	mw "myApp/internal/http/middleware"
	"myApp/internal/storage"
	"myApp/internal/view"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/csrf"
	"github.com/jmoiron/sqlx"
)

// New — собирает HTTP-роутер из модулей (middleware, DB, маршруты, шаблоны)
func New(cfg core.Config, db *sqlx.DB, csrfKey []byte) (http.Handler, error) { // ← Добавлен db параметр
	tpl, err := initTemplates()
	if err != nil {
		return nil, err
	}

	r := chi.NewRouter()

	// Подключаем middleware в правильном порядке
	useDatabaseMiddleware(r, db) // ← DB в контекст (первым!)
	useBaseMiddleware(r, cfg)
	useSecurityMiddleware(r, cfg)
	useCSRF(r, cfg, csrfKey)
	serveStatic(r)
	registerRoutes(r, tpl)

	return r, nil
}

/* ---------- Инициализация ---------- */

// initTemplates — создаёт экземпляр шаблонизатора
func initTemplates() (*view.Templates, error) {
	return view.New()
}

/* ---------- Database Middleware ---------- */

// useDatabaseMiddleware — добавляет *sqlx.DB в контекст каждого запроса
// DB доступен во всех handlers через storage.GetDBFromContext()
func useDatabaseMiddleware(r *chi.Mux, db *sqlx.DB) {
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Помещает DB в контекст запроса через storage.CtxDBKey
			ctx := context.WithValue(r.Context(), storage.CtxDBKey{}, db)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
}

/* ---------- Middleware ---------- */

// useBaseMiddleware — базовые middleware (nonce, логи, таймауты)
func useBaseMiddleware(r *chi.Mux, cfg core.Config) {
	r.Use(withNonce())                                   // Nonce для CSP (первым!)
	r.Use(mw.TrustedProxy([]string{"127.0.0.1", "::1"})) // Доверенные прокси
	r.Use(middleware.RequestID)                          // Уникальный ID запроса
	r.Use(middleware.RealIP)                             // Реальный IP клиента
	r.Use(middleware.Logger)                             // Логирование запросов
	r.Use(middleware.Recoverer)                          // Восстановление после паники
	r.Use(middleware.Timeout(cfg.RequestTimeout))        // Таймаут запроса
}

// useSecurityMiddleware — CSP, X-Frame-Options, HSTS (при HTTPS)
func useSecurityMiddleware(r *chi.Mux, cfg core.Config) {
	r.Use(mw.SecureHeaders()) // Заголовки безопасности
	if cfg.Secure {
		r.Use(mw.HSTS(cfg.Env == "prod")) // HSTS только при HTTPS и в продакшене
	}
}

// useCSRF — CSRF-защита для форм (OWASP A02)
func useCSRF(r *chi.Mux, cfg core.Config, csrfKey []byte) {
	r.Use(csrf.Protect(
		csrfKey,
		csrf.Secure(cfg.Secure),             // Cookie только по HTTPS
		csrf.SameSite(csrf.SameSiteLaxMode), // SameSite=Lax
		csrf.HttpOnly(true),                 // Cookie HttpOnly
		csrf.Path("/"),                      // CSRF для всех путей
	))
	// Предупреждение в не-продакшене без HTTPS
	if !cfg.Secure && cfg.Env != "prod" {
		core.LogError("CSRF работает без HTTPS в не-продакшен среде", nil)
	}
}

// withNonce — middleware для генерации nonce для CSP
// Добавляет случайный nonce в контекст для шаблонов
func withNonce() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nonce, err := generateNonce()
			if err != nil {
				core.LogError("Ошибка генерации nonce", map[string]interface{}{"error": err.Error()})
				core.Fail(w, r, core.Internal("Ошибка генерации nonce", err))
				return
			}
			// Nonce доступен в шаблонах через core.CtxNonce
			ctx := context.WithValue(r.Context(), core.CtxNonce, nonce)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

/* ---------- Статика и маршруты ---------- */

// serveStatic — обслуживает статические файлы из web/assets
func serveStatic(r *chi.Mux) {
	fs := http.StripPrefix("/assets/", http.FileServer(http.Dir("web/assets")))
	r.Handle("/assets/*", fs)
}

// registerRoutes — регистрирует маршруты приложения
// Все handlers получают DB из контекста автоматически
func registerRoutes(r *chi.Mux, tpl *view.Templates) {
	r.Get("/", handler.Home(tpl))
	r.Get("/catalog", handler.Catalog(tpl))
	r.Get("/product/{id}", handler.Product(tpl))

	r.Get("/form", handler.FormIndex(tpl))
	r.Post("/form", handler.FormSubmit(tpl))

	r.Get("/about", handler.About(tpl))

	r.Get("/debug", handler.Debug)
	r.Get("/catalog/json", handler.CatalogJSON())

	r.NotFound(handler.NotFound(tpl))
}

/* ---------- Утилиты ---------- */

// generateNonce — генерирует криптографически стойкий nonce для CSP
func generateNonce() (string, error) {
	b := make([]byte, 16) // 16 байт = 128 бит энтропии
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}
