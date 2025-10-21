package handler

// about.go
import (
	"net/http"

	"myApp/internal/core"
	"myApp/internal/view"

	"github.com/gin-gonic/gin"
)

// About — обработчик страницы "О нас" (OWASP A03: Injection)
func About(tpl *view.Templates) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := tpl.Render(c, "about", "О нас", nil); err != nil {
			core.LogError("Ошибка рендеринга шаблона about", map[string]interface{}{
				"error": err.Error(),
				"path":  c.Request.URL.Path,
			})
			c.String(http.StatusInternalServerError, "Ошибка отображения страницы")
			return
		}
	}
}
