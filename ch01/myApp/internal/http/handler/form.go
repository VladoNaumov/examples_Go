package handler

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"myApp/internal/core"
	"myApp/internal/view"

	"github.com/go-playground/validator/v10"
	"github.com/microcosm-cc/bluemonday"
)

// FormData определяет структуру данных формы
type FormData struct {
	Name    string `validate:"required,min=2,max=100"` // Имя пользователя (2–100 символов)
	Email   string `validate:"required,email"`         // Email (валидный формат)
	Message string `validate:"required,max=2000"`      // Сообщение (до 2000 символов)
}

// FormView — структура для передачи данных в шаблон
type FormView struct {
	Form   FormData          // Введённые данные
	Errors map[string]string // Ошибки валидации
	OK     bool              // Флаг успешной отправки
}

var (
	validate  = validator.New()        // Валидатор структуры
	sanitizer = bluemonday.UGCPolicy() // Санитизатор ввода
)

// FormIndex возвращает обработчик отображения формы (OWASP A03: Injection)
func FormIndex(tpl *view.Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ok := r.URL.Query().Get("ok") == "1"

		data := FormView{
			Form:   FormData{},
			Errors: map[string]string{},
			OK:     ok,
		}

		if err := tpl.Render(w, r, "form", "Форма", data); err != nil {
			core.LogError("Ошибка рендеринга шаблона form", map[string]interface{}{
				"error": err.Error(),
				"path":  r.URL.Path,
			})
			http.Error(w, "Ошибка отображения страницы", http.StatusInternalServerError)
			return
		}
	}
}

// FormSubmit возвращает обработчик отправки формы (OWASP A03, A05)
func FormSubmit(tpl *view.Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Метод не разрешён", http.StatusMethodNotAllowed)
			return
		}

		// Ограничивает размер тела запроса (1 MB)
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Некорректный запрос", http.StatusBadRequest)
			return
		}

		// Санитизация данных формы
		f := FormData{
			Name:    sanitizer.Sanitize(strings.TrimSpace(r.Form.Get("name"))),
			Email:   sanitizer.Sanitize(strings.TrimSpace(r.Form.Get("email"))),
			Message: sanitizer.Sanitize(strings.TrimSpace(r.Form.Get("message"))),
		}

		// Проверка валидации
		errs := map[string]string{}
		if err := validate.Struct(f); err != nil {
			var verrs validator.ValidationErrors
			if errors.As(err, &verrs) {
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
				log.Printf("INFO: Ошибка валидации формы: %v", errs)
			} else {
				// Обработка других типов ошибок валидации
				var invErr *validator.InvalidValidationError
				if errors.As(err, &invErr) {
					errs["form"] = "Неверная конфигурация валидации"
					core.LogError("InvalidValidationError", map[string]interface{}{"error": invErr.Error()})
				} else {
					errs["form"] = "Ошибка валидации"
					core.LogError("Неожиданная ошибка валидации", map[string]interface{}{"error": err.Error()})
				}
			}
		}

		// Если есть ошибки — рендерим форму снова с сообщениями
		if len(errs) > 0 {
			data := FormView{
				Form:   f,
				Errors: errs,
				OK:     false,
			}

			w.WriteHeader(http.StatusBadRequest)
			if err := tpl.Render(w, r, "form", "Форма", data); err != nil {
				core.LogError("Ошибка рендеринга шаблона form", map[string]interface{}{
					"error": err.Error(),
					"path":  r.URL.Path,
				})
				http.Error(w, "Ошибка отображения страницы", http.StatusInternalServerError)
			}
			return
		}

		// PRG-паттерн: перенаправляем на форму с флагом успеха
		http.Redirect(w, r, "/form?ok=1", http.StatusSeeOther)
	}
}
