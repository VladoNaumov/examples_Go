package handler

import (
	"net/http"

	"myApp/internal/core"
	"myApp/internal/view"

	"github.com/gin-gonic/gin"
)

// Home — обработчик главной страницы (OWASP A03: Injection)
func Home(tpl *view.Templates) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Пример данных для шаблона (можешь убрать/заменить)
		data := map[string]any{
			"Welcome": "Добро пожаловать в магазин!",
			"Lang":    "ru",
		}

		// Рендерим шаблон "home"
		if err := tpl.Render(c, "home", "Главная", data); err != nil {
			core.LogError("Ошибка рендеринга шаблона home", map[string]interface{}{
				"error": err.Error(),
				"path":  c.Request.URL.Path,
			})

			// Отдаём 500 — стандартный ответ
			c.String(http.StatusInternalServerError, "Ошибка отображения страницы")
			return
		}
	}
}
