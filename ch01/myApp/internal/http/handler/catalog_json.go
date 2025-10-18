package handler

import (
	"encoding/json"
	"net/http"

	"myApp/internal/core"
	"myApp/internal/storage"
)

// CatalogJSON возвращает каталог товаров в формате JSON
func CatalogJSON() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Проверяем метод запроса
		if r.Method != http.MethodGet {
			http.Error(w, "Только GET разрешён", http.StatusMethodNotAllowed)
			return
		}

		// Получаем БД из контекста
		db := storage.GetDBFromContext(r.Context())
		if db == nil {
			core.LogError("DB недоступна в контексте", nil)
			core.Fail(w, r, core.Internal("Внутренняя ошибка", nil))
			return
		}

		// Загружаем товары
		items, err := storage.ListAllProducts(r.Context(), db)
		if err != nil {
			core.LogError("Ошибка загрузки каталога", map[string]interface{}{
				"error": err.Error(),
			})
			core.Fail(w, r, core.Internal("Ошибка каталога", err))
			return
		}

		// Настраиваем заголовки для JSON
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		// Устанавливаем статус 200
		w.WriteHeader(http.StatusOK)

		// Кодируем и отправляем JSON
		if err := json.NewEncoder(w).Encode(items); err != nil {
			// Логируем ошибку, но не прерываем ответ (уже начали отправку)
			core.LogError("Ошибка кодирования JSON", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}
}
