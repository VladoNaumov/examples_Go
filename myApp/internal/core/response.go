package core

// response.go
import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

// JSON отправляет JSON-ответ (OWASP A09).
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		LogError("JSON encoding failed", map[string]interface{}{"error": err.Error()})
	}
}

// NoContent: Предназначена для возврата HTTP 204 (No Content), но в проекте нет таких endpoint'ов.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// ProblemDetail — RFC7807 для ошибок API (OWASP A04).
type ProblemDetail struct {
	Type     string            `json:"type"`
	Title    string            `json:"title"`
	Status   int               `json:"status"`
	Detail   string            `json:"detail"`
	Instance string            `json:"instance"`
	Code     string            `json:"code"`
	Fields   map[string]string `json:"fields,omitempty"`
}

// Fail отправляет RFC7807-ответ (OWASP A04, A09).
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
	LogError("Request failed", logFields)

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
