package view

import (
	"fmt"
	"html/template"
	"net/http"

	"myApp/internal/core"

	"github.com/gorilla/csrf"
)

// 🔧 Шаблонная система приложения (View Layer)

// Этот пакет отвечает за работу с HTML-шаблонами:
//  • парсинг (однократная загрузка файлов)
//  • рендеринг страниц
//  • внедрение CSRF-токена и CSP nonce для защиты

// 🧩 Templates — хранилище всех HTML-шаблонов в памяти
type Templates struct {
	templates map[string]*template.Template // ключ — имя страницы
}

// 📦 PageData — структура, передаваемая в шаблоны
type PageData struct {
	Title     string        // Заголовок страницы
	CSRFField template.HTML // Скрытое поле <input> с CSRF-токеном
	Nonce     string        // CSP nonce — случайный токен для защиты inline-скриптов
	Data      interface{}   // Пользовательские данные (формы, товары и т.д.)
}

// Инициализация шаблонов

// New — парсит layout, partials и страницы один раз при старте приложения.
// Возвращает готовую структуру Templates со всеми шаблонами.
func New() (*Templates, error) {
	// Общие layout и частичные шаблоны, которые подключаются ко всем страницам
	layouts := []string{
		"web/templates/layouts/base.gohtml",
		"web/templates/partials/nav.gohtml",
		"web/templates/partials/footer.gohtml",
	}

	// Карта всех страниц и их файлов
	pages := map[string][]string{
		"home":     {"web/templates/pages/home.gohtml"},
		"about":    {"web/templates/pages/about.gohtml"},
		"form":     {"web/templates/pages/form.gohtml"},
		"catalog":  {"web/templates/pages/catalog.gohtml"},
		"product":  {"web/templates/pages/show_product.gohtml"},
		"notfound": {"web/templates/pages/404.gohtml"},
	}

	// Контейнер для всех шаблонов
	t := &Templates{templates: make(map[string]*template.Template)}

	// Парсим каждый шаблон страницы вместе с layout
	for name, pageFiles := range pages {
		files := append(layouts, pageFiles...)
		tpl, err := template.ParseFiles(files...)
		if err != nil {
			return nil, fmt.Errorf("ошибка парсинга шаблона %s: %w", name, err)
		}
		t.templates[name] = tpl
	}

	return t, nil
}

// 🎨 Рендеринг HTML-шаблонов

// Render — безопасно отрисовывает HTML-шаблон и добавляет CSRF и CSP-защиту.
// Используется всеми HTTP-обработчиками (handlers).
func (t *Templates) Render(
	w http.ResponseWriter,
	r *http.Request,
	templateName string, // имя шаблона (например, "home" или "form")
	title string, // заголовок страницы
	data interface{}, // любые пользовательские данные
) error {

	// 1️ Проверяем, что нужный шаблон существует
	tpl, ok := t.templates[templateName]
	if !ok {
		core.LogError("Шаблон не найден", map[string]interface{}{
			"template": templateName,
		})
		core.Fail(w, r, core.Internal("Шаблон не найден", nil))
		return fmt.Errorf("шаблон не найден: %s", templateName)
	}

	// 2️ Извлекаем CSP nonce (одноразовый токен) из контекста запроса
	//    Он нужен для того, чтобы браузер выполнял только безопасные inline-скрипты
	nonce, _ := r.Context().Value(core.CtxNonce).(string)
	if nonce == "" {
		core.LogError("Nonce не найден в контексте", nil)
		core.Fail(w, r, core.Internal("Ошибка безопасности", nil))
		return fmt.Errorf("nonce не найден")
	}

	// 3️ Устанавливаем заголовок Content-Type
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// 4️ Формируем данные, передаваемые в шаблон
	page := PageData{
		Title:     title,
		CSRFField: csrf.TemplateField(r), // добавляем скрытый CSRF <input>
		Nonce:     nonce,                 // передаём CSP nonce
		Data:      data,                  // любые пользовательские данные
	}

	// 5️ Рендерим шаблон base, в который вложены partials и конкретная страница
	if err := tpl.ExecuteTemplate(w, "base", page); err != nil {
		core.LogError("Ошибка рендеринга шаблона", map[string]interface{}{
			"template": templateName,
			"error":    err.Error(),
		})
		core.Fail(w, r, core.Internal("Ошибка шаблона", err))
		return fmt.Errorf("рендеринг шаблона %s: %w", templateName, err)
	}

	return nil // Всё прошло успешно
}

// 🧠 Кратко о том, как это работает
//
// 1. При запуске сервера → вызывается view.New(), шаблоны загружаются в память.
// 2. Каждый handler вызывает tpl.Render(w, r, "имя", "заголовок", storage).
// 3. Render добавляет:
//      - CSRF-токен (gorilla/csrf)
//      - CSP nonce (для защиты inline-скриптов)
//      - Заголовок Content-Type
// 4. Шаблон "base.gohtml" получает PageData и отрисовывает:
//      {{ .Title }}        → Заголовок страницы
//      {{ .CSRFField }}    → <input type="hidden" name="_csrf" ...>
//      {{ .Nonce }}        → nonce в meta-тегах CSP (.Nonce — это одноразовый токен (random string), который вставляется в HTML-страницу для защиты от XSS-атак).
//      {{ .Data }}         → твои данные (форма, товары и т.д.)
