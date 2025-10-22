package core

// security.go — отвечает за установку безопасных HTTP-заголовков.

import (
	"github.com/gin-gonic/gin"
)

// -----------------------------------------------------------
// SecureHeaders — middleware: CSP с nonce + безопасные заголовки
// -----------------------------------------------------------

func SecureHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")
		c.Writer.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Writer.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		c.Next()
	}
}

// CSP возвращает middleware для установки заголовка Content-Security-Policy.
// CSP — это система защиты браузера от XSS, инъекций скриптов и стилей.
// Она говорит браузеру: "Разреши загружать только то, что я укажу".
//
// Как работает:
// 1. На каждый запрос генерируется nonce (случайная строка).
// 2. Nonce кладётся в контекст и в CSP-политику.
// 3. В HTML вставляем <script nonce="..."> или <style nonce="..."> — только они работают.
// 4. Всё остальное (инлайн style="", внешние скрипты без разрешения) — блокируется.
//
// Директивы (настройки):
//   default-src      — что разрешено по умолчанию
//   style-src        — откуда стили (файлы, инлайн)
//   script-src       — откуда скрипты
//   img-src          — картинки
//   font-src         — шрифты
//   object-src       — плагины (flash и т.п.)
//   frame-ancestors  — кто может вставить нас в <iframe>
//   base-uri         — откуда <base>
//
// Значения:
//   'self'           — только с нашего домена
//   'none'           — ничего
//   https://cdn...   — конкретный внешний источник
//   'nonce-ABC123'   — только с этим nonce
//   data:            — data-uri (например, base64-картинки)
//
// --- ВАРИАНТ 1: БАЗОВЫЙ (БЕЗОПАСНЫЙ, КАК СЕЙЧАС) ---

func CSPBasic() gin.HandlerFunc {
	return func(c *gin.Context) {
		nonce, _ := c.Request.Context().Value(CtxNonce).(string)

		c.Header("Content-Security-Policy",
			"default-src 'self'; "+
				"base-uri 'self'; "+
				"object-src 'none'; "+
				"frame-ancestors 'none'; "+
				"connect-src 'self'; "+ // Разрешает AJAX, Fetch, WebSockets только на свой домен
				"worker-src 'self'; "+ // Разрешает Web/Service Workers только со своего домена
				"style-src 'self' https://cdn.jsdelivr.net 'nonce-"+nonce+"'; "+
				"script-src 'self' https://cdn.jsdelivr.net 'nonce-"+nonce+"'; "+
				"img-src 'self' data: https://cdn.jsdelivr.net; "+
				"font-src 'self' https://cdn.jsdelivr.net; ")
		c.Next()
	}
}

// --- ВАРИАНТ 2: РАЗРЕШИТЬ ВСЕ ИНЛАЙН СТИЛИ (НЕБЕЗОПАСНО!) ---
// Добавляем 'unsafe-inline' — браузер разрешит style="..."
// НО: это ослабляет защиту! Используй только для тестов.

func CSPAllowInlineStyles() gin.HandlerFunc {
	return func(c *gin.Context) {
		nonce, _ := c.Request.Context().Value(CtxNonce).(string)

		c.Header("Content-Security-Policy",
			"default-src 'self'; "+
				"style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net 'nonce-"+nonce+"'; "+ // ← 'unsafe-inline'
				"script-src 'self' 'nonce-"+nonce+"'; "+
				"img-src 'self' data:; ")
		c.Next()
	}
}

// --- ВАРИАНТ 3: СТРОГИЙ — ТОЛЬКО СВОИ РЕСУРСЫ, БЕЗ CDN ---
// Никаких внешних библиотек. Только локальные файлы и nonce.

func CSPStrictLocalOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		nonce, _ := c.Request.Context().Value(CtxNonce).(string)

		c.Header("Content-Security-Policy",
			"default-src 'self'; "+
				"style-src 'self' 'nonce-"+nonce+"'; "+ // только свои CSS и nonce
				"script-src 'self' 'nonce-"+nonce+"'; "+ // только свои JS и nonce
				"img-src 'self' data:; "+ // картинки только с сервера или data:
				"font-src 'self'; "+ // шрифты только свои
				"object-src 'none'; "+
				"frame-ancestors 'none'; ")
		c.Next()
	}
}

// --- ВАРИАНТ 4: ДЛЯ РАЗРАБОТКИ — ОТКЛЮЧИТЬ CSP СОВСЕМ ---
// Внимание: НЕ ИСПОЛЬЗУЙ В ПРОДАКШЕНЕ!

func CSPDisabled() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Ничего не ставим — CSP не будет
		c.Next()
	}
}

/*

### 🧠 Краткое объяснение, зачем нужны эти заголовки

| Заголовок                            | Назначение                                                      | Пример значения                                                                 |
| ------------------------------------ | --------------------------------------------------------------- | ------------------------------------------------------------------------------- |
| **Content-Security-Policy**          | Контроль разрешённых источников контента (CSS, JS, изображения) | `"default-src 'self'; script-src 'self' https://cdn.jsdelivr.net 'nonce-XYZ';"` |
| **X-Content-Type-Options**           | Запрещает MIME type sniffing (XSS-защита)                       | `"nosniff"`                                                                     |
| **X-Frame-Options**                  | Запрещает встраивание сайта в iframe                            | `"DENY"`                                                                        |
| **Referrer-Policy**                  | Управляет, что передаётся в Referer при переходах               | `"strict-origin-when-cross-origin"`                                             |
| **Permissions-Policy**               | Запрещает доступ к камере, микрофону, геолокации                | `"camera=(), microphone=(), geolocation=()"`                                    |
| **Strict-Transport-Security (HSTS)** | Обязывает браузер использовать HTTPS                            | `"max-age=31536000; includeSubDomains"`                                         |

*/
