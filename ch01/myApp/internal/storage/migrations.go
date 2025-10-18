package storage

// migrations.go
import (
	"fmt"
	"os"
	"strings"

	"myApp/internal/core"

	"github.com/jmoiron/sqlx"
)

const (
	EnableMigrations = false                       // true → выполнять миграцию
	MigrationFile    = "migrations/001_schema.sql" // путь к файлу миграции
)

type Migrations struct {
	db *sqlx.DB
}

func NewMigrations(db *sqlx.DB) *Migrations {
	return &Migrations{db: db}
}

func (m *Migrations) RunMigrations() error {
	if !EnableMigrations {
		core.LogInfo("Миграции отключены (EnableMigrations=false)", nil)
		return nil
	}

	core.LogInfo("Начало выполнения миграции", map[string]interface{}{
		"file": MigrationFile,
	})

	content, err := os.ReadFile(MigrationFile)
	if err != nil {
		core.LogError("Ошибка чтения файла миграции", map[string]interface{}{
			"file":  MigrationFile,
			"error": err.Error(),
		})
		fmt.Println("Ошибка применения миграции")
		return err
	}

	sqlText := cleanSQL(string(content))
	stmts := strings.Split(sqlText, ";")

	executed := 0
	for _, raw := range stmts {
		stmt := strings.TrimSpace(raw)
		if stmt == "" {
			continue
		}
		if _, err := m.db.Exec(stmt); err != nil {
			core.LogError("Ошибка SQL", map[string]interface{}{
				"sql":   stmt,
				"error": err.Error(),
			})
			fmt.Println(" Ошибка применения миграции")
			return fmt.Errorf("ошибка выполнения SQL: %w", err)
		}
		executed++
	}

	core.LogInfo("Миграция успешно применена", map[string]interface{}{
		"file":       MigrationFile,
		"statements": executed,
	})
	fmt.Println(" Миграция применена")
	return nil
}

// cleanSQL — убирает комментарии и пустые строки
func cleanSQL(sql string) string {
	lines := strings.Split(sql, "\n")
	var clean []string
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "" || strings.HasPrefix(t, "--") {
			continue
		}
		clean = append(clean, t)
	}
	return strings.Join(clean, "\n")
}
