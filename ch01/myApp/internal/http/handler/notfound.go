package handler

import (
	"myApp/internal/core"
	"myApp/internal/view"
	"net/http"

	"github.com/gin-gonic/gin"
)

// NotFound — страница 404 (OWASP A03)
func NotFound(tpl *view.Templates) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Ставим 404 до рендера (чтобы статус ушёл даже если шаблон успешен)
		c.Status(http.StatusNotFound)

		if err := tpl.Render(c, "notfound", "Страница не найдена", nil); err != nil {
			core.LogError("Ошибка рендеринга шаблона notfound", map[string]interface{}{
				"error": err.Error(),
				"path":  c.Request.URL.Path,
			})
			// Фолбэк, если шаблон упал
			c.String(http.StatusInternalServerError, "Ошибка отображения страницы")
			return
		}
	}
}
