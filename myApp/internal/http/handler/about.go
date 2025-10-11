package handler

//about.go
import (
	"net/http"

	"myApp/internal/view"
)

func About(tpl *view.Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tpl.Render(w, r, "about", "О нас", nil)
	}
}
