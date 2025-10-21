package handler

// catalog.go
import (
	"myApp/internal/core"
	"myApp/internal/storage"
	"myApp/internal/view"

	"github.com/gin-gonic/gin"
)

// Catalog — отображает каталог товаров из MySQL
func Catalog(tpl *view.Templates) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Достаём *sqlx.DB из контекста запроса (мы его туда положили в middleware withNonceAndDB)
		db := storage.GetDBFromContext(c.Request.Context())
		if db == nil {
			core.LogError("DB недоступна в контексте", nil)
			core.FailC(c, core.Internal("Внутренняя ошибка", nil))
			return
		}

		items, err := storage.ListAllProducts(c.Request.Context(), db)
		if err != nil {
			core.LogError("Ошибка загрузки каталога", map[string]interface{}{"error": err.Error()})
			core.FailC(c, core.Internal("Ошибка каталога", err))
			return
		}

		if err := tpl.Render(c, "catalog", "Каталог товаров", items); err != nil {
			core.LogError("Ошибка рендеринга catalog", map[string]interface{}{"error": err.Error()})
			core.FailC(c, core.Internal("Ошибка отображения", err))
			return
		}
	}
}
