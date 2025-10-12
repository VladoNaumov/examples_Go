package view

import (
	"html/template"
	"net/http"

	"myApp/internal/core"

	"github.com/gorilla/csrf"
)

// Templates — структура для хранения шаблонов.
type Templates struct {
	templates map[string]*template.Template
}

// PageData — унифицированная структура для шаблонов (OWASP A03, A07).
type PageData struct {
	Title     string
	CSRFField template.HTML
	Nonce     string
	Data      interface{}
}

// New инициализирует шаблоны (OWASP A05).
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

// Render рендерит шаблон с данными (OWASP).
func (t *Templates) Render(w http.ResponseWriter, r *http.Request, templateName string, title string, data interface{}) {
	tpl, ok := t.templates[templateName]
	if !ok {
		core.Fail(w, r, core.Internal("template not found: "+templateName, nil))
		return
	}

	nonce, _ := r.Context().Value(core.CtxNonce).(string)
	if nonce == "" {
		core.Fail(w, r, core.Internal("nonce not found in context", nil))
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tpl.ExecuteTemplate(w, "base", PageData{
		Title:     title,
		CSRFField: csrf.TemplateField(r),
		Nonce:     nonce,
		Data:      data,
	}); err != nil {
		core.LogError("Template rendering failed", map[string]interface{}{
			"template": templateName,
			"error":    err.Error(),
		})
		core.Fail(w, r, core.Internal("template error", err))
	}
}
