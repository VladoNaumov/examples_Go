package handler

// form.go (Gin)
import (
	"errors"
	"log"
	"net/http"
	"strings"

	"myApp/internal/core"
	"myApp/internal/view"

	"github.com/gin-gonic/gin"
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

// FormIndex — GET-страница формы (OWASP A03: Injection)
func FormIndex(tpl *view.Templates) gin.HandlerFunc {
	return func(c *gin.Context) {
		ok := c.Query("ok") == "1"

		data := FormView{
			Form:   FormData{},
			Errors: map[string]string{},
			OK:     ok,
		}

		if err := tpl.Render(c, "form", "Форма", data); err != nil {
			core.LogError("Ошибка рендеринга шаблона form", map[string]interface{}{
				"error": err.Error(),
				"path":  c.Request.URL.Path,
			})
			c.String(http.StatusInternalServerError, "Ошибка отображения страницы")
			return
		}
	}
}

// FormSubmit — POST-обработчик отправки формы (OWASP A03, A05)
func FormSubmit(tpl *view.Templates) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost {
			c.String(http.StatusMethodNotAllowed, "Метод не разрешён")
			return
		}

		// Ограничиваем размер тела запроса (1 MB)
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)

		// Явно парсим форму (можно использовать c.Request.ParseForm() или c.ShouldBind)
		if err := c.Request.ParseForm(); err != nil {
			c.String(http.StatusBadRequest, "Некорректный запрос")
			return
		}

		// Санитизация данных формы
		f := FormData{
			Name:    sanitizer.Sanitize(strings.TrimSpace(c.Request.Form.Get("name"))),
			Email:   sanitizer.Sanitize(strings.TrimSpace(c.Request.Form.Get("email"))),
			Message: sanitizer.Sanitize(strings.TrimSpace(c.Request.Form.Get("message"))),
		}

		// Валидация
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

		// Если есть ошибки — возвращаем 400 и рендерим форму с ошибками
		if len(errs) > 0 {
			data := FormView{
				Form:   f,
				Errors: errs,
				OK:     false,
			}
			c.Status(http.StatusBadRequest) // статус до рендера
			if err := tpl.Render(c, "form", "Форма", data); err != nil {
				core.LogError("Ошибка рендеринга шаблона form", map[string]interface{}{
					"error": err.Error(),
					"path":  c.Request.URL.Path,
				})
				c.String(http.StatusInternalServerError, "Ошибка отображения страницы")
			}
			return
		}

		// PRG-паттерн: редирект на GET /form?ok=1
		c.Redirect(http.StatusSeeOther, "/form?ok=1")
	}
}
