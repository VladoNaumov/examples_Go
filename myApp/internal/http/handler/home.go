package handler

//home.go
import (
	"net/http"

	"myApp/internal/view"
)

func Home(tpl *view.Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tpl.Render(w, r, "home", "Главная", nil)
	}
}
