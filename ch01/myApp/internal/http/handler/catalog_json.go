package handler

import (
	"net/http"

	"myApp/internal/core"
	"myApp/internal/storage"

	"github.com/gin-gonic/gin"
)

// CatalogJSON — JSON-эндпоинт каталога (Gin-версия)
func CatalogJSON() gin.HandlerFunc {
	return func(c *gin.Context) {
		db := storage.GetDBFromContext(c.Request.Context())
		if db == nil {
			core.LogError("DB недоступна в контексте", nil)
			core.FailC(c, core.Internal("Внутренняя ошибка", nil))
			return
		}

		items, err := storage.ListAllProducts(c.Request.Context(), db)
		if err != nil {
			core.LogError("Ошибка загрузки каталога (JSON)", map[string]interface{}{"error": err.Error()})
			core.FailC(c, core.Internal("Ошибка каталога", err))
			return
		}

		// Можно через ваш helper, если он есть: core.JSON(c, http.StatusOK, items)
		c.JSON(http.StatusOK, items)
	}
}
