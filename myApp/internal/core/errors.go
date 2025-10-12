package core

// errors.go
import (
	"errors"
	"fmt"
	"net/http"
)

// AppError — единый контейнер для ошибок приложения.
type AppError struct {
	Code    string            // Машинный код ("validation", "not_found", etc.)
	Status  int               // HTTP-статус
	Message string            // Сообщение для клиента
	Err     error             // Внутренняя ошибка
	Fields  map[string]string // Поле -> текст ошибки
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s (%d): %s: %v", e.Code, e.Status, e.Message, e.Err)
	}
	return fmt.Sprintf("%s (%d): %s", e.Code, e.Status, e.Message)
}

func (e *AppError) Unwrap() error { return e.Err }

// Фабрики ошибок (OWASP A05).
func BadRequest(msg string, fields map[string]string) *AppError {
	if len(fields) > 10 {
		fields = map[string]string{"form": "Too many validation errors"}
	}
	return &AppError{Code: "bad_request", Status: http.StatusBadRequest, Message: msg, Fields: fields}
}

// Не используется, так как 404 обрабатывается через handler.NotFound с рендером шаблона.
func NotFound(msg string) *AppError {
	return &AppError{Code: "not_found", Status: http.StatusNotFound, Message: msg}
}

// Не используется, так как в проекте нет логики ограничения доступа (403).
func Forbidden(msg string) *AppError {
	return &AppError{Code: "forbidden", Status: http.StatusForbidden, Message: msg}
}

// Не используется, так как нет аутентификации (401).
func Unauthorized(msg string) *AppError {
	return &AppError{Code: "unauthorized", Status: http.StatusUnauthorized, Message: msg}
}

func Internal(msg string, err error) *AppError {
	return &AppError{Code: "internal", Status: http.StatusInternalServerError, Message: msg, Err: err}
}

// From приводит ошибку к AppError (OWASP A09).
func From(err error) *AppError {
	var ae *AppError
	if errors.As(err, &ae) {
		return ae
	}
	if err == nil {
		return nil
	}
	return Internal("internal error", err)
}
