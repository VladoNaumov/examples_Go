package handler

//home.go
import (
	"myApp/internal/core"
	"net/http"

	"myApp/internal/view"
)

// Home возвращает обработчик для главной страницы (OWASP A03: Injection)
func Home(tpl *view.Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Рендерим шаблон "home" с заголовком
		if err := tpl.Render(w, r, "home", "Главная", nil); err != nil {
			core.LogError("Ошибка рендеринга шаблона home", map[string]interface{}{
				"error": err.Error(),
				"path":  r.URL.Path,
			})

			// В случае ошибки — стандартный ответ
			http.Error(w, "Ошибка отображения страницы", http.StatusInternalServerError)
			return
		}
	}
}
