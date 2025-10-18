package core

//response.go
import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

// JSON отправляет JSON-ответ с указанным статусом HTTP (OWASP A09: Security Logging and Monitoring Failures)
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		LogError("Ошибка кодирования JSON", map[string]interface{}{"error": err.Error()})
	}
}

// ProblemDetail определяет структуру ответа об ошибке в формате RFC 7807 (OWASP A04: Design Flaws)
type ProblemDetail struct {
	Type     string            `json:"type"`             // Тип ошибки (URI)
	Title    string            `json:"title"`            // Название HTTP-статуса
	Status   int               `json:"status"`           // HTTP-статус
	Detail   string            `json:"detail"`           // Детали ошибки
	Instance string            `json:"instance"`         // Путь запроса
	Code     string            `json:"code"`             // Машинный код ошибки
	Fields   map[string]string `json:"fields,omitempty"` // Поля с ошибками (для валидации)
}

// Fail отправляет ответ об ошибке в формате RFC 7807 и логирует её (OWASP A04, A09)
func Fail(w http.ResponseWriter, r *http.Request, err error) {
	ae := From(err)
	requestID := middleware.GetReqID(r.Context())
	logFields := map[string]interface{}{
		"request_id": requestID,
		"path":       r.URL.Path,
		"code":       ae.Code,
		"status":     ae.Status,
		"message":    ae.Message,
		"fields":     ae.Fields,
		"error":      ae.Err,
	}
	LogError("Ошибка обработки запроса", logFields)

	problem := ProblemDetail{
		Type:     "/errors/" + ae.Code,
		Title:    http.StatusText(ae.Status),
		Status:   ae.Status,
		Detail:   ae.Message,
		Instance: r.URL.Path,
		Code:     ae.Code,
		Fields:   ae.Fields,
	}
	JSON(w, ae.Status, problem)
}
