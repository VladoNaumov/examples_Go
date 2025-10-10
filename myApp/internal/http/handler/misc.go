package handler

import (
	"html/template"
	"net/http"

	"github.com/gorilla/csrf"
	"myApp/internal/core"
)

// Глобальные шаблоны для 404 (OWASP A05).
var notFoundTpl = template.Must(template.ParseFiles(
	"web/templates/layouts/base.gohtml",
	"web/templates/partials/nav.gohtml",
	"web/templates/partials/footer.gohtml",
	"web/templates/pages/404.gohtml",
))

// Health — healthcheck с core.JSON (OWASP A09).
func Health(w http.ResponseWriter, r *http.Request) {
	core.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// NotFound — 404 с шаблоном.
func NotFound(w http.ResponseWriter, r *http.Request) {
	nonce := r.Context().Value("nonce").(string)
	w.WriteHeader(http.StatusNotFound)
	data := PageData{
		Title:     "Страница не найдена",
		CSRFField: csrf.TemplateField(r),
		Nonce:     nonce,
	}
	if err := notFoundTpl.ExecuteTemplate(w, "base", data); err != nil {
		core.Fail(w, r, core.Internal("template error", err))
	}
}
