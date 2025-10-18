package handler

// catalog.go
import (
	"net/http"

	"myApp/internal/core"
	"myApp/internal/data"
	"myApp/internal/view"
)

// Catalog отображает каталог товаров из MySQL
func Catalog(tpl *view.Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		db := data.GetDBFromContext(r.Context())
		if db == nil {
			core.LogError("DB недоступна в контексте", nil)
			core.Fail(w, r, core.Internal("Внутренняя ошибка", nil))
			return
		}

		items, err := data.ListAllProducts(r.Context(), db)
		if err != nil {
			core.LogError("Ошибка загрузки каталога", map[string]interface{}{
				"error": err.Error(),
			})
			core.Fail(w, r, core.Internal("Ошибка каталога", err))
			return
		}

		// Render теперь возвращает error
		err = tpl.Render(w, r, "catalog", "Каталог товаров", items)
		if err != nil {
			core.LogError("Ошибка рендеринга catalog", map[string]interface{}{
				"error": err.Error(),
			})
			core.Fail(w, r, core.Internal("Ошибка отображения", err))
			return
		}
	}
}
