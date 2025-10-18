package handler

import (
	"net/http"

	"myApp/internal/core"
	"myApp/internal/view"
)

// ProductView — структура данных для отображения страницы товара.
// Передаётся в шаблон через PageData.Data.
type ProductView struct {
	Name        string            // Название товара
	Article     string            // Артикул / SKU
	Price       string            // Цена с валютой
	ImageURL    string            // URL изображения
	Short       string            // Краткое описание
	Description string            // Полное описание
	BackURL     string            // Ссылка для кнопки "Назад"
	Attributes  map[string]string // Таблица характеристик
}

// Product — обработчик страницы одного товара (OWASP A03: Injection-safe rendering)
func Product(tpl *view.Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 🔹 Подготовим данные товара (в реальном проекте здесь будет запрос из БД)
		data := ProductView{
			Name:        "Товар №1",
			Article:     "ART-0001",
			Price:       "29,90 €",
			ImageURL:    "https://picsum.photos/seed/p1/800/800",
			Short:       "Увлажняющее средство для волос с натуральными маслами. Придаёт мягкость, блеск и облегчает расчёсывание.",
			Description: "Лёгкая текстура и свежий аромат. Подходит для ежедневного применения и всех типов волос.",
			BackURL:     "/", // ссылка «Назад»
			Attributes: map[string]string{
				"Бренд":     "BrandName",
				"Страна":    "Finland",
				"Объём":     "250 ml",
				"Категория": "Уход за волосами",
			},
		}

		// 🔹 Рендерим страницу с шаблоном "product"
		if err := tpl.Render(w, r, "product", "Товар — Encanta Shop", data); err != nil {
			core.LogError("Ошибка рендеринга product", map[string]interface{}{
				"error": err.Error(),
				"path":  r.URL.Path,
			})
			http.Error(w, "Ошибка отображения страницы", http.StatusInternalServerError)
			return
		}
	}
}
