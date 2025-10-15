package view

//view.go
import (
	"fmt"
	"html/template"
	"net/http"

	"myApp/internal/core"

	"github.com/gorilla/csrf"
)

// Templates хранит предварительно загруженные HTML-шаблоны
type Templates struct {
	templates map[string]*template.Template // Карта шаблонов по именам
}

// PageData определяет данные для рендеринга шаблонов
type PageData struct {
	Title     string        // Заголовок страницы
	CSRFField template.HTML // Поле CSRF-токена
	Nonce     string        // Nonce для Content Security Policy
	Data      interface{}   // Дополнительные данные для шаблона
}

// New инициализирует шаблоны из файлов
func New() (*Templates, error) {
	layouts := []string{
		"web/templates/layouts/base.gohtml",
		"web/templates/partials/nav.gohtml",
		"web/templates/partials/footer.gohtml",
	}
	pages := map[string][]string{
		"home":     {"web/templates/pages/home.gohtml"},
		"about":    {"web/templates/pages/about.gohtml"},
		"form":     {"web/templates/pages/form.gohtml"},
		"catalog":  {"web/templates/pages/catalog.gohtml"}, // Добавлен catalog
		"notfound": {"web/templates/pages/404.gohtml"},
	}

	t := &Templates{templates: make(map[string]*template.Template)}
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

// Render рендерит шаблон с данными, включая CSRF-токен и nonce
// Возвращает error для обработки в handlers
func (t *Templates) Render(w http.ResponseWriter, r *http.Request, templateName string, title string, data interface{}) error {
	tpl, ok := t.templates[templateName]
	if !ok {
		core.LogError("Шаблон не найден", map[string]interface{}{
			"template": templateName,
		})
		core.Fail(w, r, core.Internal("Шаблон не найден", nil))
		return fmt.Errorf("шаблон не найден: %s", templateName)
	}

	nonce, _ := r.Context().Value(core.CtxNonce).(string)
	if nonce == "" {
		core.LogError("Nonce не найден в контексте", nil)
		core.Fail(w, r, core.Internal("Ошибка безопасности", nil))
		return fmt.Errorf("nonce не найден")
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := tpl.ExecuteTemplate(w, "base", PageData{
		Title:     title,
		CSRFField: csrf.TemplateField(r),
		Nonce:     nonce,
		Data:      data,
	}); err != nil {
		core.LogError("Ошибка рендеринга шаблона", map[string]interface{}{
			"template": templateName,
			"error":    err.Error(),
		})
		core.Fail(w, r, core.Internal("Ошибка шаблона", err))
		return fmt.Errorf("рендеринг шаблона %s: %w", templateName, err)
	}

	return nil // Успешно отрендерено
}
