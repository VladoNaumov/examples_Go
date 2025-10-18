package handler

//about.go
import (
	"myApp/internal/core"
	"net/http"

	"myApp/internal/view"
)

// About возвращает обработчик для страницы "О нас" (OWASP A03: Injection)
func About(tpl *view.Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Рендерим шаблон "about" с заголовком
		if err := tpl.Render(w, r, "about", "О нас", nil); err != nil {
			core.LogError("Ошибка рендеринга шаблона about", map[string]interface{}{
				"error": err.Error(),
				"path":  r.URL.Path,
			})
			http.Error(w, "Ошибка отображения страницы", http.StatusInternalServerError)
			return
		}
	}
}
