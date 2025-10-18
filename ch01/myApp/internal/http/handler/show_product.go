package handler

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"myApp/internal/core"
	"myApp/internal/storage"
	"myApp/internal/view"

	"github.com/go-chi/chi/v5"
)

// Product — детальная страница товара, архитектура как в Catalog
func Product(tpl *view.Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Извлекаем DB из контекста (как в Catalog)
		db := storage.GetDBFromContext(r.Context())
		if db == nil {
			core.LogError("DB недоступна в контексте", nil)
			core.Fail(w, r, core.Internal("Внутренняя ошибка", nil))
			return
		}

		// 2. Извлекаем ID из URL
		idStr := chi.URLParam(r, "id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			core.LogError("Неверный ID товара", map[string]interface{}{
				"id":    idStr,
				"error": err.Error(),
			})
			core.Fail(w, r, core.Internal("Неверный ID товара", err))
			return
		}

		product, err := storage.GetProductByID(r.Context(), db, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				core.LogError("Товар не найден", map[string]interface{}{
					"id": id,
				})
				core.Fail(w, r, core.Internal("Товар не найден", nil))
				return
			}
			core.LogError("Ошибка загрузки товара", map[string]interface{}{
				"id":    id,
				"error": err.Error(),
			})
			core.Fail(w, r, core.Internal("Ошибка товара", err))
			return
		}

		// 4. Рендерим как в Catalog
		err = tpl.Render(w, r, "product", product.Name, product)
		if err != nil {
			core.LogError("Ошибка рендеринга product", map[string]interface{}{
				"id":    id,
				"error": err.Error(),
			})
			core.Fail(w, r, core.Internal("Ошибка отображения", err))
			return
		}
	}
}
