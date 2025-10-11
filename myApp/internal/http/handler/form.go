package handler

//form.go
import (
	"net/http"
	"strings"

	"myApp/internal/core"
	"myApp/internal/view"

	"github.com/go-playground/validator/v10"
	"github.com/microcosm-cc/bluemonday"
)

type FormData struct {
	Name    string `validate:"required,min=2,max=100"`
	Email   string `validate:"required,email"`
	Message string `validate:"required,max=2000"`
}

type FormView struct {
	Form   FormData
	Errors map[string]string
	OK     bool // Added for success flag.
}

var (
	validate  = validator.New()
	sanitizer = bluemonday.UGCPolicy()
)

func FormIndex(tpl *view.Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ok := r.URL.Query().Get("ok") == "1"
		data := FormView{
			Form:   FormData{},
			Errors: map[string]string{},
			OK:     ok,
		}
		tpl.Render(w, r, "form", "Форма", data)
	}
}

func FormSubmit(tpl *view.Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
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
			data := FormView{
				Form:   f,
				Errors: errs,
				OK:     false,
			}
			tpl.Render(w, r, "form", "Форма", data)
			return
		}

		http.Redirect(w, r, "/form?ok=1", http.StatusSeeOther)
	}
}
