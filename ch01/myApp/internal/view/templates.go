package view

// internal/view/templates.go — Система HTML-шаблонизации с поддержкой безопасности.

import (
	"fmt"
	"html/template"

	"myApp/internal/core"

	"github.com/gin-gonic/gin"
	csrf "github.com/utrack/gin-csrf"
)

// Templates — хранилище всех HTML-шаблонов в памяти.
// Ключ — имя страницы (например, "home").
type Templates struct {
	templates map[string]*template.Template
}

// PageData — структура данных, передаваемая в шаблоны.
type PageData struct {
	Title     string        // Заголовок страницы
	CSRFField template.HTML // Скрытое поле <input> с CSRF-токеном (для защиты форм)
	Nonce     string        // CSP nonce для inline-скриптов/стилей (для защиты от XSS)
	Data      any           // Пользовательские данные, специфичные для страницы
}

// New — парсит layout, partials и страницы один раз при старте приложения и кэширует.
func New() (*Templates, error) {
	// Общие layout и частичные шаблоны, которые включаются во все страницы
	layouts := []string{
		"web/templates/layouts/base.gohtml",
		"web/templates/partials/nav.gohtml",
		"web/templates/partials/footer.gohtml",
	}

	// Страницы -> файлы
	pages := map[string][]string{
		"home":     {"web/templates/pages/home.gohtml"},
		"about":    {"web/templates/pages/about.gohtml"},
		"form":     {"web/templates/pages/form.gohtml"},
		"catalog":  {"web/templates/pages/catalog.gohtml"},
		"product":  {"web/templates/pages/show_product.gohtml"},
		"notfound": {"web/templates/pages/404.gohtml"},
	}

	t := &Templates{templates: make(map[string]*template.Template)}

	for name, pageFiles := range pages {
		// Собираем полный список файлов: layout + partials + страница
		files := append([]string{}, layouts...)
		files = append(files, pageFiles...)

		tpl, err := template.ParseFiles(files...)
		if err != nil {
			return nil, fmt.Errorf("ошибка парсинга шаблона %q: %w", name, err)
		}
		t.templates[name] = tpl
	}

	return t, nil
}

// Render — отрисовывает HTML-шаблон с добавлением данных безопасности (CSRF/CSP).
// Принимает *gin.Context, чтобы брать токены и nonce, которые были добавлены middleware.
func (t *Templates) Render(
	c *gin.Context,
	templateName string, // "home" | "form" | ...
	title string,
	data any,
) error {
	// 1) Проверка наличия шаблона
	tpl, ok := t.templates[templateName]
	if !ok {
		core.LogError("Шаблон не найден", map[string]interface{}{"template": templateName})
		return fmt.Errorf("шаблон не найден: %s", templateName)
	}

	// 2) 🛠️ ИСПРАВЛЕНИЕ: Достаём CSP nonce ТОЛЬКО из request.Context.
	// Nonce добавляется в request.Context через middleware withNonceAndDB.
	nonce := ""
	if v, ok := c.Request.Context().Value(core.CtxNonce).(string); ok {
		nonce = v
	}

	if nonce == "" {
		// Если nonce не найден — это критическая ошибка: не сработал middleware безопасности
		core.LogError("CSP Nonce не найден в контексте запроса", nil)
		return fmt.Errorf("nonce не найден: критическая ошибка безопасности")
	}

	// 3) Заголовок контента
	// Важно явно указать Content-Type, чтобы избежать MIME-sniffing.
	c.Header("Content-Type", "text/html; charset=utf-8")

	// 4) CSRF: берём токен и сами собираем скрытое поле.
	token := csrf.GetToken(c)
	if token == "" {
		// Это может произойти, если сессия не установлена; логируем, но продолжаем рендеринг.
		core.LogError("CSRF токен пуст. Форма будет отправлена без защиты.", nil)
	}

	// Безопасное формирование HTML-поля с CSRF-токеном, используя HTMLEscapeString.
	csrfField := template.HTML(
		fmt.Sprintf(`<input type="hidden" name="%s" value="%s">`,
			"csrf_token",                     // Имя поля, ожидаемое gin-csrf
			template.HTMLEscapeString(token), // Экранирование для безопасности
		),
	)

	// 5) Собираем PageData и рендерим
	page := PageData{
		Title:     title,
		CSRFField: csrfField,
		Nonce:     nonce,
		Data:      data,
	}

	// ExecuteTemplate пишет прямо в ResponseWriter, используя корневой шаблон "base"
	if err := tpl.ExecuteTemplate(c.Writer, "base", page); err != nil {
		core.LogError("Ошибка рендеринга шаблона", map[string]interface{}{
			"template": templateName,
			"error":    err.Error(),
		})
		// Возвращаем ошибку, чтобы вызывающий хендлер мог ее обработать (например, 500)
		return fmt.Errorf("рендеринг шаблона %s: %w", templateName, err)
	}
	return nil
}

// 🧠 Как это работает в нашей версии (Gin + utrack/gin-csrf):
//
// 1) При запуске сервера вызывается view.New() — шаблоны парсятся один раз и хранятся в памяти.
//
// 2) Каждый Gin-хендлер вызывает tpl.Render(c, "имя", "заголовок", data).
//    Render получает nonce из Gin-контекста (кладётся middleware) и подготавливает PageData.
//
// 3) CSRF: токен берём из utrack/gin-csrf: token := csrf.GetToken(c).
//    Скрытое поле собираем вручную:
//       <input type="hidden" name="csrf_token" value="...">
//    (имя параметра — "csrf_token" по умолчанию у utrack/gin-csrf).
//
// 4) CSP: nonce пробрасывается в PageData.Nonce и используется в шаблоне:
//       <script nonce="{{ .Nonce }}">...</script>
//       <style  nonce="{{ .Nonce }}">...</style>
//    SecureHeaders() формирует CSP с разрешением по nonce.
//
// 5) Контент-тайп: Render ставит заголовок "Content-Type: text/html; charset=utf-8".
//
// 6) В base.gohtml должен быть корневой шаблон с именем "base" ({{ define "base" }} ... {{ end }}),
//    в который дочерние страницы подключаются через {{ template }} или {{ block }}.

/*
<form method="POST" action="/form">
  {{ .CSRFField }}
  <input type="text" name="name">
  <input type="email" name="email">
  <textarea name="message"></textarea>
  <button type="submit">Отправить</button>
</form>

*/
