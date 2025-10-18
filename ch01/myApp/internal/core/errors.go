package core

//errors.go
import (
	"errors"
	"fmt"
	"net/http"
)

// AppError представляет ошибку приложения с кодом, HTTP-статусом и дополнительной информацией (OWASP A05: Security Misconfiguration)
type AppError struct {
	Code    string            // Машинный код ошибки (например, "validation", "not_found")
	Status  int               // HTTP-статус для ответа клиенту
	Message string            // Сообщение для клиента
	Err     error             // Внутренняя ошибка (если есть)
	Fields  map[string]string // Поле -> текст ошибки (для валидации)
}

// Error возвращает строковое представление ошибки
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s (%d): %s: %v", e.Code, e.Status, e.Message, e.Err)
	}
	return fmt.Sprintf("%s (%d): %s", e.Code, e.Status, e.Message)
}

// Unwrap возвращает вложенную ошибку для обработки цепочки ошибок
func (e *AppError) Unwrap() error { return e.Err }

// BadRequest создаёт ошибку для некорректного запроса (HTTP 400) (OWASP A05)
func BadRequest(msg string, fields map[string]string) *AppError {
	// Ограничивает количество полей ошибок для предотвращения перегрузки
	if len(fields) > 10 {
		fields = map[string]string{"form": "Слишком много ошибок валидации"}
	}
	return &AppError{Code: "bad_request", Status: http.StatusBadRequest, Message: msg, Fields: fields}
}

// NotFound создаёт ошибку для ресурса, который не найден (HTTP 404) (OWASP A05)
// Примечание: не используется, так как 404 обрабатывается через handler.NotFound с рендером шаблона
func NotFound(msg string) *AppError {
	return &AppError{Code: "not_found", Status: http.StatusNotFound, Message: msg}
}

// Forbidden создаёт ошибку для запрета доступа (HTTP 403) (OWASP A05)
// Примечание: не используется, так как в проекте нет логики ограничения доступа
func Forbidden(msg string) *AppError {
	return &AppError{Code: "forbidden", Status: http.StatusForbidden, Message: msg}
}

// Unauthorized создаёт ошибку для неавторизованного доступа (HTTP 401) (OWASP A05)
// Примечание: не используется, так как в проекте нет аутентификации
func Unauthorized(msg string) *AppError {
	return &AppError{Code: "unauthorized", Status: http.StatusUnauthorized, Message: msg}
}

// Internal создаёт ошибку для внутренней серверной ошибки (HTTP 500) (OWASP A05)
func Internal(msg string, err error) *AppError {
	return &AppError{Code: "internal", Status: http.StatusInternalServerError, Message: msg, Err: err}
}

// From преобразует ошибку в AppError, возвращая Internal при неизвестной ошибке (OWASP A09: Security Logging and Monitoring Failures)
func From(err error) *AppError {
	var ae *AppError
	if errors.As(err, &ae) {
		return ae
	}
	if err == nil {
		return nil
	}
	return Internal("внутренняя ошибка", err)
}
