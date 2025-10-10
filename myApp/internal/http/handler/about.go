package handler

// about.go

import (
	"html/template"
	"net/http"

	"github.com/gorilla/csrf"
	"myApp/internal/core"
)

// Глобальные шаблоны (OWASP A05).
var aboutTpl = template.Must(template.ParseFiles(
	"web/templates/layouts/base.gohtml",
	"web/templates/partials/nav.gohtml",
	"web/templates/partials/footer.gohtml",
	"web/templates/pages/about.gohtml",
))

func About(w http.ResponseWriter, r *http.Request) {
	nonce := r.Context().Value("nonce").(string)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := PageData{
		Title:     "О нас",
		CSRFField: csrf.TemplateField(r),
		Nonce:     nonce,
	}
	if err := aboutTpl.ExecuteTemplate(w, "base", data); err != nil {
		core.Fail(w, r, core.Internal("template error", err))
	}
}
