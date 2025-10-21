package handler

// product.go (Gin)
import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"myApp/internal/core"
	"myApp/internal/storage"
	"myApp/internal/view"

	"github.com/gin-gonic/gin"
)

// Product — детальная страница товара
func Product(tpl *view.Templates) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1) Достаём DB из контекста (кладётся в middleware withNonceAndDB)
		db := storage.GetDBFromContext(c.Request.Context())
		if db == nil {
			core.LogError("DB недоступна в контексте", nil)
			core.FailC(c, core.Internal("Внутренняя ошибка", nil))
			return
		}

		// 2) Берём :id из маршрута (/product/:id) и валидируем
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil || id <= 0 {
			core.LogError("Неверный ID товара", map[string]interface{}{
				"id":    idStr,
				"error": err,
			})
			// 400 Bad Request в формате RFC7807
			core.FailC(c, &core.AppError{
				Code:    "bad_request",
				Status:  http.StatusBadRequest,
				Message: "Неверный ID товара",
				Err:     err,
			})
			return
		}

		// 3) Достаём товар из БД
		product, err := storage.GetProductByID(c.Request.Context(), db, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				core.LogError("Товар не найден", map[string]interface{}{"id": id})
				// 404 Not Found в формате RFC7807
				core.FailC(c, &core.AppError{
					Code:    "not_found",
					Status:  http.StatusNotFound,
					Message: "Товар не найден",
				})
				return
			}
			core.LogError("Ошибка загрузки товара", map[string]interface{}{
				"id":    id,
				"error": err.Error(),
			})
			core.FailC(c, core.Internal("Ошибка загрузки товара", err))
			return
		}

		// 4) Рендерим шаблон "product" (заголовок — имя товара)
		if err := tpl.Render(c, "product", product.Name, product); err != nil {
			core.LogError("Ошибка рендеринга product", map[string]interface{}{
				"id":    id,
				"error": err.Error(),
			})
			core.FailC(c, core.Internal("Ошибка отображения", err))
			return
		}
	}
}
