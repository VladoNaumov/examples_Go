package handler

import (
	"net/http"

	"myApp/internal/core"

	"github.com/gin-gonic/gin"
)

// Debug — возвращает отладочную информацию в JSON
func Debug(c *gin.Context) {
	info := map[string]interface{}{
		"request": map[string]interface{}{
			"method":  c.Request.Method,
			"url":     c.Request.URL.String(),
			"headers": c.Request.Header,
			"remote":  c.Request.RemoteAddr,
		},
		"response": map[string]interface{}{
			"content_type": "application/json",
			"status":       http.StatusOK,
			"note":         "OK!",
		},
	}

	core.JSON(c, http.StatusOK, info)
}
