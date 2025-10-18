package storage

// internal/storage/ products_repo.go
import (
	"context"
	"myApp/internal/core"
	"time"

	"github.com/jmoiron/sqlx"
)

type Product struct {
	ID         string    `db:"id" json:"id"`
	CategoryID *string   `db:"category_id" json:"category_id,omitempty"`
	Name       string    `db:"name" json:"name"`
	Article    string    `db:"article" json:"article"`
	Price      float64   `db:"price" json:"price"`
	ImageAlt   *string   `db:"image_alt" json:"image_alt,omitempty"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
}

func ListAllProducts(ctx context.Context, db *sqlx.DB) ([]Product, error) {
	const q = `
		SELECT p.id, p.name, p.price, p.image_alt
		FROM products p
		ORDER BY p.name ASC`

	var items []Product

	// context.Context — “контейнер” для управления временем жизни операции и передачи метаданных.
	// db.SelectContext - Возвращает много строк (срез структур)
	if err := db.SelectContext(ctx, &items, q); err != nil {
		core.LogError("list all products", map[string]interface{}{
			"query": q,
			"error": err.Error(),
		})
		return nil, err
	}
	return items, nil
}

// GetProductByID — находим товар по ID
// context.Context — “контейнер” для управления временем жизни операции и передачи метаданных.
// db.GetContext - Возвращает одну строку (один объект).
func GetProductByID(ctx context.Context, db *sqlx.DB, id int) (*Product, error) {
	var p Product

	const q = `
		SELECT id, name, article, price, image_alt
		FROM products 
		WHERE id = ?`

	if err := db.GetContext(ctx, &p, q, id); err != nil {
		core.LogError("get product by id", map[string]interface{}{
			"id":    id,
			"error": err.Error(),
			"query": q,
		})
		return nil, err
	}
	return &p, nil
}
