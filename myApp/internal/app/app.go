package app

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"time"

	"myApp/internal/core"
	"myApp/internal/http/handler"
	mw "myApp/internal/http/middleware"
	"myApp/internal/view"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/csrf"
)

// собирает приложение (OWASP A05).
func New(cfg core.Config, csrfKey []byte) (http.Handler, error) {
	tpl, err := view.New()
	if err != nil {
		return nil, err
	}

	r := chi.NewRouter()

	// Генерируем nonce на КАЖДЫЙ запрос и кладём в контекст.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nonce, err := generateNonce()
			if err != nil {
				core.LogError("Failed to generate nonce", map[string]interface{}{"error": err.Error()})
				nonce = ""
			}
			ctx := context.WithValue(r.Context(), core.CtxNonce, nonce)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})

	// Middleware
	r.Use(mw.TrustedProxy([]string{"127.0.0.1", "::1"}))
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(15 * time.Second))

	// Secure headers теперь берёт nonce из контекста (см. пункт 3).
	r.Use(mw.SecureHeaders())

	if cfg.Secure {
		r.Use(mw.HSTS(cfg.Env == "prod"))
	}

	r.Use(csrf.Protect(
		csrfKey,
		csrf.Secure(cfg.Secure),
		csrf.SameSite(csrf.SameSiteLaxMode),
		csrf.HttpOnly(true),
		csrf.Path("/"),
	))
	if !cfg.Secure {
		core.LogError("CSRF running without HTTPS in non-prod", nil)
	}

	// Статические файлы (кэш в NGINX)
	r.Handle("/assets/*", http.StripPrefix("/assets/", http.FileServer(http.Dir("web/assets"))))

	// Роуты
	r.Get("/", handler.Home(tpl))
	r.Get("/about", handler.About(tpl))
	r.Get("/form", handler.FormIndex(tpl))
	r.Post("/form", handler.FormSubmit(tpl))
	r.Get("/healthz", handler.Health)
	r.NotFound(handler.NotFound(tpl))

	return r, nil
}

// generateNonce создаёт nonce для CSP (OWASP).
func generateNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}
