package handler

import (
	"myApp/internal/core"
	"myApp/internal/view"
	"net/http"
)

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
