package handler

//misc.go
import (
	"net/http"

	"myApp/internal/core"
	"myApp/internal/view"
)

func Health(w http.ResponseWriter, r *http.Request) {
	core.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func NotFound(tpl *view.Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		tpl.Render(w, r, "notfound", "Страница не найдена", nil)
	}
}
