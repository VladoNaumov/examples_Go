package handler

//misc.go
import (
	"net/http"

	"myApp/internal/core"
	"myApp/internal/view"
)

// Health возвращает обработчик для проверки состояния сервера (OWASP A09)
func Health(w http.ResponseWriter, r *http.Request) {
	info := map[string]interface{}{
		"request": map[string]interface{}{
			"method":  r.Method,
			"url":     r.URL.String(),
			"headers": r.Header,
			"remote":  r.RemoteAddr,
		},
		"response": map[string]interface{}{
			"content_type": "application/json",
			"status":       http.StatusOK,
			"note":         "Это ответ, который вы сейчас видите",
		},
	}

	core.JSON(w, http.StatusOK, info)
}

// NotFound возвращает обработчик для страницы 404 (OWASP A03)
func NotFound(tpl *view.Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Устанавливаем статус 404
		w.WriteHeader(http.StatusNotFound)

		// Рендерим шаблон "notfound" с обработкой ошибки
		if err := tpl.Render(w, r, "notfound", "Страница не найдена", nil); err != nil {
			core.LogError("Ошибка рендеринга шаблона notfound", map[string]interface{}{
				"error": err.Error(),
				"path":  r.URL.Path,
			})
		}
	}
}
