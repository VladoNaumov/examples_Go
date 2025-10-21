package app

// internal/app/app.go
import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"os"
	"time"

	"myApp/internal/core"
	"myApp/internal/http/handler"
	"myApp/internal/storage"
	"myApp/internal/view"

	"github.com/gin-contrib/requestid"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	csrf "github.com/utrack/gin-csrf"
)

// ВАЖНО: CSRF secret (долгоживущий ключ) ≠ CSP nonce (случайное значение на КАЖДЫЙ запрос).
// CSRF-защита — для форм; CSP nonce — для разрешения инлайн <style>/<script nonce="...">.

// initTemplates — инициализация шаблонов.
func initTemplates() (*view.Templates, error) {
	return view.New()
}

// New — главный конструктор Gin + вся инициализация middleware.
func New(cfg core.Config, db *sqlx.DB, csrfKey []byte) (http.Handler, error) {
	tpl, err := initTemplates()
	if err != nil {
		return nil, err
	}

	r := gin.New()

	// Базовые логи/восстановление паник
	r.Use(gin.Logger(), gin.Recovery())

	// Trust proxy только локально (если нужен внешний NGINX — добавь его IP тут)
	_ = r.SetTrustedProxies([]string{"127.0.0.1", "::1"})

	// Корреляция запросов
	r.Use(requestid.New())

	// Таймаут запроса (отсекаем "висящие" клиенты)
	r.Use(RequestTimeout(cfg.RequestTimeout))

	// Кладём nonce и DB в контекст запроса — это нужно ДО установки CSP.
	r.Use(withNonceAndDB(db))

	// Security заголовки (X-Frame-Options, X-Content-Type-Options и пр.)
	// ВАЖНО: Убедись, что core.SecureHeaders() НЕ добавляет Content-Security-Policy.
	// CSP ниже выставляется отдельным middleware, чтобы подставить nonce.
	r.Use(core.SecureHeaders())

	// Строгая CSP С НОНСОМ. Должна идти ПОСЛЕ withNonceAndDB.
	r.Use(CSP())

	// HSTS только для HTTPS; в продакшене можно включить preload (внутри core.HSTS)
	if cfg.Secure {
		r.Use(core.HSTS(cfg.Env == "prod"))
	}

	// Безопасные cookie-сессии (HttpOnly, SameSite, Secure=prod)
	store := cookie.NewStore(csrfKey)
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
		Secure:   cfg.Secure,
		SameSite: http.SameSiteLaxMode,
	})
	r.Use(sessions.Sessions("mysession", store))

	// CSRF защита форм. Секрет независим от CSP nonce.
	r.Use(csrf.Middleware(csrf.Options{
		Secret:    string(csrfKey),
		ErrorFunc: csrfError,
	}))

	// Статика (если папка есть) — ВКЛЮЧЕНО
	serveStatic(r)

	// Роуты
	registerRoutes(r, tpl)

	return r, nil
}

// RequestTimeout — безопасный таймаут для всего запроса.
func RequestTimeout(d time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d <= 0 {
			c.Next()
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), d)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)
		c.Next()

		// Если контекст протух и ещё ничего не было записано в ответ — вернём 408
		if ctx.Err() != nil && !c.Writer.Written() {
			c.AbortWithStatusJSON(http.StatusRequestTimeout, gin.H{"error": "timeout"})
		}
	}
}

// withNonceAndDB — генерирует CSP nonce и кладёт вместе с *sqlx.DB в контекст запроса.
func withNonceAndDB(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		nonce, err := generateNonce()
		if err != nil {
			core.LogError("nonce error", map[string]interface{}{"error": err})
			core.FailC(c, core.Internal("nonce", err))
			return
		}
		ctx := context.WithValue(c.Request.Context(), core.CtxNonce, nonce)
		ctx = context.WithValue(ctx, storage.CtxDBKey{}, db)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// CSP — выставляет жёсткую CSP с тем самым nonce из контекста.
// ВАЖНО: nonce действует ДЛЯ <style nonce="..."> и <script nonce="...">,
// НО НЕ для атрибутов style="...". Атрибуты будут блокироваться, пока не уберёшь их.
func CSP() gin.HandlerFunc {
	return func(c *gin.Context) {
		nonce, _ := c.Request.Context().Value(core.CtxNonce).(string)

		// Собери политику под свои нужды. Ниже — безопасный базовый вариант.
		// Если используешь сторонние CDN — явно перечисляй их.
		c.Header("Content-Security-Policy",
			"default-src 'self'; "+
				"base-uri 'self'; "+
				"object-src 'none'; "+
				"frame-ancestors 'none'; "+
				// Стили: только свои, jsdelivr и инлайн <style nonce="...">.
				// Атрибуты style="" всё равно будут ЗАПРЕЩЕНЫ.
				"style-src 'self' https://cdn.jsdelivr.net 'nonce-"+nonce+"'; "+
				// Скрипты: свои, jsdelivr и инлайн <script nonce="...">.
				"script-src 'self' https://cdn.jsdelivr.net 'nonce-"+nonce+"'; "+
				// Картинки: свои, data: (иконки/инлайн PNG), и, при необходимости, CDN.
				"img-src 'self' data: https://cdn.jsdelivr.net; "+
				// Шрифты с CDN (если нужны)
				"font-src 'self' https://cdn.jsdelivr.net; ")

		c.Next()
	}
}

// csrfError — единообразный ответ на невалидный CSRF-токен.
func csrfError(c *gin.Context) {
	core.FailC(c, core.Internal("CSRF invalid", nil))
}

// serveStatic — раздача файлов из web/assets на /assets.
func serveStatic(r *gin.Engine) {
	if _, err := os.Stat("web/assets"); os.IsNotExist(err) {
		core.LogError("web/assets missing", nil)
		return
	}
	r.Static("/assets", "web/assets")
}

// registerRoutes — маршруты приложения.
func registerRoutes(r *gin.Engine, tpl *view.Templates) {
	r.GET("/", handler.Home(tpl))
	r.GET("/catalog", handler.Catalog(tpl))
	r.GET("/product/:id", handler.Product(tpl))
	r.GET("/form", handler.FormIndex(tpl))
	r.POST("/form", handler.FormSubmit(tpl))
	r.GET("/about", handler.About(tpl))
	r.GET("/debug", handler.Debug)
	r.GET("/catalog/json", handler.CatalogJSON())
	r.NoRoute(handler.NotFound(tpl))

	// Тихо гасим запросы от Chrome DevTools
	r.GET("/.well-known/appspecific/com.chrome.devtools.json", func(c *gin.Context) {
		c.Status(http.StatusNoContent) // 204
	})
}

// generateNonce — создаёт 16 байт криптографически стойкой случайности и кодирует в Base64.
// Ошибку ОБЯЗАТЕЛЬНО проверяем (golangci-lint errcheck).
func generateNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil { // ← обработка ошибки обязательна
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}
