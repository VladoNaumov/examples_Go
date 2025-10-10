package handler

// form.go
import (
	"html/template"
	"net/http"
	"strings"

	"myApp/internal/core"

	"github.com/go-playground/validator/v10"
	"github.com/gorilla/csrf"
	"github.com/microcosm-cc/bluemonday"
)

type PageData struct {
	Title     string
	CSRFField template.HTML
	OK        bool
	Nonce     string // Для CSP (OWASP A03).
}

type FormData struct {
	Name    string `validate:"required,min=2,max=100"`
	Email   string `validate:"required,email"`
	Message string `validate:"required,max=2000"`
}

type FormView struct {
	PageData
	Form   FormData
	Errors map[string]string
}

// Глобальные шаблоны и валидатор (OWASP A05).
var (
	tpl = template.Must(template.ParseFiles(
		"web/templates/layouts/base.gohtml",
		"web/templates/partials/nav.gohtml",
		"web/templates/partials/footer.gohtml",
		"web/templates/pages/form.gohtml",
	))
	validate  = validator.New()
	sanitizer = bluemonday.UGCPolicy()
)

// FormIndex рендерит форму (GET).
func FormIndex(w http.ResponseWriter, r *http.Request) {
	nonce := r.Context().Value("nonce").(string) // Получаем nonce из middleware.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	ok := r.URL.Query().Get("ok") == "1"
	data := FormView{
		PageData: PageData{
			Title:     "Форма",
			CSRFField: csrf.TemplateField(r),
			OK:        ok,
			Nonce:     nonce,
		},
		Form:   FormData{},
		Errors: map[string]string{},
	}
	if err := tpl.ExecuteTemplate(w, "base", data); err != nil {
		core.Fail(w, r, core.Internal("template error", err))
	}
}

// FormSubmit обрабатывает отправку формы (POST).
func FormSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB (OWASP A05).
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	f := FormData{
		Name:    sanitizer.Sanitize(strings.TrimSpace(r.Form.Get("name"))),
		Email:   sanitizer.Sanitize(strings.TrimSpace(r.Form.Get("email"))),
		Message: sanitizer.Sanitize(strings.TrimSpace(r.Form.Get("message"))),
	}

	errs := map[string]string{}
	if err := validate.Struct(f); err != nil {
		if verrs, ok := err.(validator.ValidationErrors); ok {
			for _, e := range verrs {
				switch e.Field() {
				case "Name":
					switch e.Tag() {
					case "required":
						errs["name"] = "Укажите имя"
					case "min":
						errs["name"] = "Имя должно быть не короче 2 символов"
					case "max":
						errs["name"] = "Слишком длинное имя (макс. 100)"
					default:
						errs["name"] = "Некорректное имя"
					}
				case "Email":
					switch e.Tag() {
					case "required":
						errs["email"] = "Укажите email"
					case "email":
						errs["email"] = "Введите корректный email"
					default:
						errs["email"] = "Некорректный email"
					}
				case "Message":
					switch e.Tag() {
					case "required":
						errs["message"] = "Напишите сообщение"
					case "max":
						errs["message"] = "Слишком длинное сообщение (макс. 2000)"
					default:
						errs["message"] = "Некорректное сообщение"
					}
				}
			}
			core.LogError("Validation failed", map[string]interface{}{"errors": errs})
		} else {
			errs["form"] = "Ошибка валидации"
			core.LogError("Unexpected validation error", map[string]interface{}{"error": err.Error()})
		}
	}

	if len(errs) > 0 {
		nonce := r.Context().Value("nonce").(string)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		data := FormView{
			PageData: PageData{
				Title:     "Форма",
				CSRFField: csrf.TemplateField(r),
				Nonce:     nonce,
			},
			Form:   f,
			Errors: errs,
		}
		if err := tpl.ExecuteTemplate(w, "base", data); err != nil {
			core.Fail(w, r, core.Internal("template error", err))
		}
		return
	}

	http.Redirect(w, r, "/form?ok=1", http.StatusSeeOther)
}
