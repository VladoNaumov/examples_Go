package handler

//debug_json.go
import (
	"net/http"

	"myApp/internal/core"
)

func Debug(w http.ResponseWriter, r *http.Request) {
	info := map[string]interface{}{
		"request": map[string]interface{}{
			"method":  r.Method,
			"url":     r.URL.String(),
			"headers": r.Header,
			"remote":  r.RemoteAddr,
		},
		"response": map[string]interface{}{
			"content_type": "application/json",
			"status":       http.StatusOK,
			"note":         "OK!",
		},
	}

	core.JSON(w, http.StatusOK, info)
}
