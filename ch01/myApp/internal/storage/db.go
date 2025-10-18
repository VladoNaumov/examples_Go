package storage

import (
	"context"
	"strings"
	"time"

	"myApp/internal/core"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

// CtxDBKey - ключ для хранения *sqlx.DB в контексте запроса
type CtxDBKey struct{}

// Константы подключения к MySQL (заменить на продакшн значения)
const (
	MySQLUser     = "root"           // Пользователь БД
	MySQLPassword = "admin"          // Пароль БД (ЗАМЕНИТЬ!)
	MySQLHost     = "localhost:3306" // Хост и порт MySQL
	MySQLDatabase = "shop"           // Имя базы данных
)

// NewDB создаёт пул подключений к MySQL с продакшн-настройками
// Инициализирует connection pool и проверяет подключение
func NewDB() (*sqlx.DB, error) {
	dsn := getMySQLDSN()

	// Подключение к MySQL
	db, err := sqlx.ConnectContext(context.Background(), "mysql", dsn)
	if err != nil {
		core.LogError("ошибка подключения к MySQL", map[string]interface{}{
			"error": err.Error(),
			"dsn":   getSanitizedDSN(dsn),
		})
		return nil, err
	}

	// Настройка connection pool для продакшена
	db.SetMaxOpenConns(25)                 // Максимум одновременных подключений
	db.SetMaxIdleConns(25)                 // Подключения в пуле ожидания
	db.SetConnMaxLifetime(5 * time.Minute) // Ротация соединений

	// Проверка подключения
	if err := db.PingContext(context.Background()); err != nil {
		//  обрабатываем ошибку закрытия, чтобы не было "Unhandled error"
		if cerr := db.Close(); cerr != nil {
			core.LogError("ошибка закрытия MySQL пула после неуспешного ping", map[string]interface{}{
				"error": cerr.Error(),
			})
		}
		core.LogError("ошибка проверки подключения MySQL", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, err
	}

	// Успешное подключение - логируем через LogInfo
	core.LogInfo("MySQL подключение успешно", map[string]interface{}{
		"host":     MySQLHost,
		"database": MySQLDatabase,
		"max_open": 25,
		"max_idle": 25,
	})

	return db, nil
}

// Close корректно закрывает пул подключений
// Вызывается при graceful shutdown приложения
func Close(db *sqlx.DB) error {
	if db == nil {
		return nil
	}

	if err := db.Close(); err != nil {
		core.LogError("ошибка закрытия MySQL пула", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}

	core.LogInfo("MySQL пул закрыт", nil)
	return nil
}

// GetDBFromContext извлекает *sqlx.DB из контекста HTTP-запроса
// Возвращает nil если DB не найдена (критическая ошибка)
func GetDBFromContext(ctx context.Context) *sqlx.DB {
	if db, ok := ctx.Value(CtxDBKey{}).(*sqlx.DB); ok {
		return db
	}
	return nil
}

// getMySQLDSN формирует строку подключения MySQL из констант
func getMySQLDSN() string {
	dsn := MySQLUser + ":" + MySQLPassword + "@tcp(" + MySQLHost + ")/" + MySQLDatabase
	dsn += "?parseTime=true"         // Парсинг времени
	dsn += "&charset=utf8mb4"        // Unicode + эмодзи
	dsn += "&timeout=5s"             // Таймаут подключения
	dsn += "&readTimeout=5s"         // Таймаут чтения
	dsn += "&writeTimeout=10s"       // Таймаут записи
	dsn += "&interpolateParams=true" // Prepared statements
	dsn += "&multiStatements=false"  // Безопасность SQL
	dsn += "&loc=Local"              // Локальная временная зона
	return dsn
}

// getSanitizedDSN удаляет пароль из DSN для логирования
func getSanitizedDSN(dsn string) string {
	if passIdx := strings.Index(dsn, ":"); passIdx > 0 {
		if atIdx := strings.Index(dsn[passIdx:], "@"); atIdx > 0 {
			return dsn[:passIdx] + ":***@tcp(" + dsn[passIdx+atIdx+1:]
		}
	}
	return dsn
}
