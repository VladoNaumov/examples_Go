package view

//views.go
import (
	"html/template"
	"myApp/internal/core"
	"net/http"

	"github.com/gorilla/csrf"
)

// Templates — структура для хранения шаблонов.
type Templates struct {
	templates map[string]*template.Template
}

// PageData — унифицированная структура для всех шаблонов (OWASP A03, A07).
type PageData struct {
	Title     string
	CSRFField template.HTML
	Nonce     string
	Data      interface{} // Для кастомных данных (например, FormView)
}

// New инициализирует шаблоны (OWASP A05).
func New() (*Templates, error) {
	// Определяем шаблоны
	layouts := []string{
		"web/templates/layouts/base.gohtml",
		"web/templates/partials/nav.gohtml",
		"web/templates/partials/footer.gohtml",
	}
	pages := map[string][]string{
		"home":     {"web/templates/pages/home.gohtml"},
		"about":    {"web/templates/pages/about.gohtml"},
		"form":     {"web/templates/pages/form.gohtml"},
		"notfound": {"web/templates/pages/404.gohtml"},
	}

	t := &Templates{templates: make(map[string]*template.Template)}
	for name, pageFiles := range pages {
		files := append(layouts, pageFiles...)
		tpl, err := template.ParseFiles(files...)
		if err != nil {
			return nil, err
		}
		t.templates[name] = tpl
	}
	return t, nil
}

// Render рендерит шаблон с данными (OWASP A03, A09).
func (t *Templates) Render(w http.ResponseWriter, r *http.Request, templateName string, title string, data interface{}) {
	tpl, ok := t.templates[templateName]
	if !ok {
		core.Fail(w, r, core.Internal("template not found: "+templateName, nil))
		return
	}

	nonce := r.Context().Value("nonce").(string)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := tpl.ExecuteTemplate(w, "base", PageData{
		Title:     title,
		CSRFField: csrf.TemplateField(r),
		Nonce:     nonce,
		Data:      data,
	})
	if err != nil {
		core.LogError("Template rendering failed", map[string]interface{}{
			"template": templateName,
			"error":    err.Error(),
		})
		core.Fail(w, r, core.Internal("template error", err))
	}
}
