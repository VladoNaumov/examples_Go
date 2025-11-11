package main

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var uploadDir = "./uploads"

var funcMap = template.FuncMap{
	"split": strings.Split,
	"join":  strings.Join,
	"add":   func(a, b int) int { return a + b },
	"slice": func(arr []string, start, end int) []string { return arr[start:end] },
	"div":   func(a int64, b float64) float64 { return float64(a) / b },
}

var tmpl *template.Template

func init() {
	tmpl = template.New("index.gohtml").Funcs(funcMap)
	tmpl = template.Must(tmpl.ParseFiles("static/index.gohtml"))
}

type File struct {
	Name      string
	IsDir     bool
	Size      int64
	URL       string
	DeleteURL string
}

type PageData struct {
	CurrentPath string
	ParentPath  string
	Items       []File
}

func main() {
	os.MkdirAll(uploadDir, os.ModePerm)

	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/mkdir", mkdirHandler)
	http.HandleFunc("/delete/", deleteHandler)
	http.Handle("/files/", http.StripPrefix("/files/", http.FileServer(http.Dir(uploadDir))))

	fmt.Println("Сервер: http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	rawPath := strings.TrimPrefix(r.URL.Path, "/")
	cleanPath := path.Clean(rawPath)
	if cleanPath == "." {
		cleanPath = ""
	}

	osPath := filepath.FromSlash(cleanPath)
	fullPath := filepath.Join(uploadDir, osPath)

	rel, err := filepath.Rel(uploadDir, fullPath)
	if err != nil || strings.Contains(rel, "..") {
		http.Error(w, "Доступ запрещён", http.StatusForbidden)
		return
	}

	stat, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Ошибка", http.StatusInternalServerError)
		}
		return
	}

	if !stat.IsDir() {
		fileURL := "/files/" + cleanPath
		http.Redirect(w, r, fileURL, http.StatusTemporaryRedirect)
		return
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		http.Error(w, "Не могу прочитать папку", http.StatusInternalServerError)
		return
	}

	var items []File
	for _, entry := range entries {
		info, _ := entry.Info()
		name := entry.Name()

		item := File{
			Name:  name,
			IsDir: entry.IsDir(),
			Size:  info.Size(),
		}

		item.URL = "/files/" + path.Join(cleanPath, name)
		item.DeleteURL = "/delete/" + path.Join(cleanPath, name)

		items = append(items, item)
	}

	parent := path.Dir(cleanPath)
	if parent == "." || parent == "/" {
		parent = ""
	}

	data := PageData{
		CurrentPath: cleanPath,
		ParentPath:  parent,
		Items:       items,
	}

	err = tmpl.ExecuteTemplate(w, "index.gohtml", data)
	if err != nil {
		log.Println("Template execute error:", err)
		http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
	}
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не разрешён", http.StatusMethodNotAllowed)
		return
	}

	dir := r.FormValue("dir")
	osDir := filepath.FromSlash(dir)
	fullDir := filepath.Join(uploadDir, osDir)

	rel, err := filepath.Rel(uploadDir, fullDir)
	if err != nil || strings.Contains(rel, "..") {
		http.Error(w, "Недопустимый путь", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Файл не выбран", http.StatusBadRequest)
		return
	}
	defer file.Close()

	r.ParseMultipartForm(100 << 20)

	dstPath := filepath.Join(fullDir, header.Filename)
	dst, err := os.Create(dstPath)
	if err != nil {
		http.Error(w, "Не удалось сохранить", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	_, err = io.Copy(dst, file)
	if err != nil {
		http.Error(w, "Ошибка копирования", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/"+dir, http.StatusSeeOther)
}

func mkdirHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не разрешён", http.StatusMethodNotAllowed)
		return
	}

	dir := r.FormValue("dir")
	name := r.FormValue("name")
	if name == "" || strings.ContainsAny(name, "/\\") {
		http.Error(w, "Недопустимое имя", http.StatusBadRequest)
		return
	}

	osDir := filepath.FromSlash(dir)
	fullPath := filepath.Join(uploadDir, osDir, name)

	rel, err := filepath.Rel(uploadDir, fullPath)
	if err != nil || strings.Contains(rel, "..") {
		http.Error(w, "Недопустимый путь", http.StatusBadRequest)
		return
	}

	if err := os.Mkdir(fullPath, os.ModePerm); err != nil {
		http.Error(w, "Не удалось создать папку", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/"+dir, http.StatusSeeOther)
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не разрешён", http.StatusMethodNotAllowed)
		return
	}

	rawPath := strings.TrimPrefix(r.URL.Path, "/delete/")
	cleanPath := path.Clean(rawPath)
	if strings.HasPrefix(cleanPath, "/") {
		cleanPath = strings.TrimPrefix(cleanPath, "/")
	}

	osPath := filepath.FromSlash(cleanPath)
	fullPath := filepath.Join(uploadDir, osPath)

	rel, err := filepath.Rel(uploadDir, fullPath)
	if err != nil || strings.Contains(rel, "..") {
		http.Error(w, "Доступ запрещён", http.StatusForbidden)
		return
	}

	if err := os.RemoveAll(fullPath); err != nil {
		http.Error(w, "Не удалось удалить", http.StatusInternalServerError)
		return
	}

	parent := path.Dir(cleanPath)
	if parent == "." || parent == "/" {
		parent = ""
	}
	http.Redirect(w, r, "/"+parent, http.StatusSeeOther)
}
