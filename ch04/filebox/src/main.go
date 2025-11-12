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

// Папка, в которой будут храниться все загруженные файлы и созданные папки.
var uploadDir = "./uploads"

// formatSize — вспомогательная функция для форматирования размера файла.
// Преобразует количество байт в более понятный формат: B, KB, MB, GB и т.д.
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// funcMap — набор пользовательских функций, которые можно использовать в HTML-шаблоне.
var funcMap = template.FuncMap{
	"split":      strings.Split,                                                         // Разделить строку по разделителю
	"join":       strings.Join,                                                          // Объединить массив строк
	"add":        func(a, b int) int { return a + b },                                   // Сложение чисел
	"slice":      func(arr []string, start, end int) []string { return arr[start:end] }, // Вырезать часть массива
	"div":        func(a int64, b float64) float64 { return float64(a) / b },            // Деление чисел
	"formatSize": formatSize,                                                            // Форматирование размера файла
}

// tmpl — шаблон HTML-страницы (index.gohtml)
var tmpl *template.Template

// init выполняется при старте программы. Загружает шаблон и связывает функции из funcMap.
func init() {
	tmpl = template.New("index.gohtml").Funcs(funcMap)
	tmpl = template.Must(tmpl.ParseFiles("static/index.gohtml"))
}

// File — структура, описывающая один элемент (файл или папку)
type File struct {
	Name          string // Имя файла или папки
	IsDir         bool   // Признак, является ли это папкой
	Size          int64  // Размер файла в байтах
	FormattedSize string // Размер файла в читаемом виде (например, "2.1 MB")
	URL           string // Ссылка для открытия
	DeleteURL     string // Ссылка для удаления
}

// PageData — структура данных, передаваемая в шаблон
type PageData struct {
	CurrentPath string // Текущая папка (для отображения пути)
	ParentPath  string // Родительская папка (для кнопки "Назад")
	Items       []File // Список файлов и папок
}

// homeHandler — обрабатывает отображение текущей папки и списка файлов
func homeHandler(w http.ResponseWriter, r *http.Request) {
	// Извлекаем путь из URL, очищаем и приводим к безопасному виду
	rawPath := strings.TrimPrefix(r.URL.Path, "/")
	cleanPath := path.Clean(rawPath)
	if cleanPath == "." {
		cleanPath = ""
	}

	osPath := filepath.FromSlash(cleanPath)
	fullPath := filepath.Join(uploadDir, osPath)

	// Проверяем, что пользователь не пытается выйти за пределы uploadDir
	rel, err := filepath.Rel(uploadDir, fullPath)
	if err != nil || strings.Contains(rel, "..") {
		http.Error(w, "Доступ запрещён: Недопустимый путь", http.StatusForbidden)
		return
	}

	stat, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
		} else {
			log.Printf("Ошибка при os.Stat(%s): %v", fullPath, err)
			http.Error(w, "Ошибка сервера при чтении файла/папки", http.StatusInternalServerError)
		}
		return
	}

	// Если это файл — перенаправляем на прямую ссылку (скачивание или просмотр)
	if !stat.IsDir() {
		fileURL := "/files/" + cleanPath
		http.Redirect(w, r, fileURL, http.StatusTemporaryRedirect)
		return
	}

	// Если это папка — читаем её содержимое
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		log.Printf("Ошибка при os.ReadDir(%s): %v", fullPath, err)
		http.Error(w, "Не могу прочитать папку", http.StatusInternalServerError)
		return
	}

	// Собираем список файлов и папок
	var items []File
	for _, entry := range entries {
		info, err := entry.Info()

		size := int64(0)
		formattedSize := "N/A"

		if err == nil {
			size = info.Size()
			formattedSize = formatSize(size)
		} else {
			log.Printf("Ошибка чтения info для %s: %v", entry.Name(), err)
		}

		name := entry.Name()

		item := File{
			Name:          name,
			IsDir:         entry.IsDir(),
			Size:          size,
			FormattedSize: formattedSize,
		}

		// Формируем ссылки
		if entry.IsDir() {
			item.URL = "/" + path.Join(cleanPath, name)
		} else {
			item.URL = "/files/" + path.Join(cleanPath, name)
		}

		item.DeleteURL = "/delete/" + path.Join(cleanPath, name)

		items = append(items, item)
	}

	// Определяем путь для кнопки "Назад"
	parent := path.Dir(cleanPath)
	if parent == "." || parent == "/" {
		parent = ""
	}

	data := PageData{
		CurrentPath: cleanPath,
		ParentPath:  parent,
		Items:       items,
	}

	// Отправляем данные в шаблон
	err = tmpl.ExecuteTemplate(w, "index.gohtml", data)
	if err != nil {
		log.Println("Template execute error:", err)
		http.Error(w, "Ошибка шаблона: "+err.Error(), http.StatusInternalServerError)
	}
}

// uploadHandler — обработчик загрузки файлов
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не разрешён", http.StatusMethodNotAllowed)
		return
	}

	// Разбор формы (до 100 МБ)
	r.ParseMultipartForm(100 << 20)

	dir := r.FormValue("dir") // Папка, куда загружаем
	osDir := filepath.FromSlash(dir)
	fullDir := filepath.Join(uploadDir, osDir)

	// Проверяем безопасность пути
	rel, err := filepath.Rel(uploadDir, fullDir)
	if err != nil || strings.Contains(rel, "..") {
		http.Error(w, "Недопустимый путь", http.StatusBadRequest)
		return
	}

	// Извлекаем файл из формы
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Файл не выбран или ошибка формы: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	dstPath := filepath.Join(fullDir, header.Filename)

	// Создаём новый файл и копируем содержимое
	dst, err := os.Create(dstPath)
	if err != nil {
		log.Printf("Ошибка при os.Create(%s): %v", dstPath, err)
		http.Error(w, "Не удалось создать файл на сервере", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	_, err = io.Copy(dst, file)
	if err != nil {
		log.Printf("Ошибка копирования в файл %s: %v", dstPath, err)
		http.Error(w, "Ошибка копирования файла", http.StatusInternalServerError)
		return
	}

	// После успешной загрузки — возвращаемся на текущую директорию
	http.Redirect(w, r, "/"+dir, http.StatusSeeOther)
}

// mkdirHandler — создаёт новую папку в текущем каталоге
func mkdirHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не разрешён", http.StatusMethodNotAllowed)
		return
	}

	dir := r.FormValue("dir")   // Текущая папка
	name := r.FormValue("name") // Имя новой папки

	// Проверка корректности имени (нельзя ../, /, \, :)
	if name == "" || strings.ContainsAny(name, "/\\:") || strings.Contains(name, "..") {
		http.Error(w, "Недопустимое имя папки (запрещены /, \\, :, ..)", http.StatusBadRequest)
		return
	}

	osDir := filepath.FromSlash(dir)
	fullPath := filepath.Join(uploadDir, osDir, name)

	// Проверка безопасности пути
	rel, err := filepath.Rel(uploadDir, fullPath)
	if err != nil || strings.Contains(rel, "..") {
		http.Error(w, "Недопустимый путь", http.StatusBadRequest)
		return
	}

	// Пытаемся создать папку
	if err := os.Mkdir(fullPath, os.ModePerm); err != nil {
		log.Printf("Ошибка при os.Mkdir(%s): %v", fullPath, err)
		http.Error(w, "Не удалось создать папку (возможно, уже существует)", http.StatusInternalServerError)
		return
	}

	// После создания — переходим в новую папку
	newPath := path.Join(dir, name)
	http.Redirect(w, r, "/"+newPath, http.StatusSeeOther)
}

// deleteHandler — удаляет файл или папку (включая всё содержимое)
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

	// Нельзя удалить корневую папку
	if cleanPath == "" || cleanPath == "." {
		http.Error(w, "Нельзя удалить корневую папку", http.StatusForbidden)
		return
	}

	osPath := filepath.FromSlash(cleanPath)
	fullPath := filepath.Join(uploadDir, osPath)

	// Проверка безопасности пути
	rel, err := filepath.Rel(uploadDir, fullPath)
	if err != nil || strings.Contains(rel, "..") {
		http.Error(w, "Доступ запрещён: Недопустимый путь", http.StatusForbidden)
		return
	}

	// Удаляем файл или папку
	if err := os.RemoveAll(fullPath); err != nil {
		log.Printf("Ошибка при os.RemoveAll(%s): %v", fullPath, err)
		http.Error(w, "Не удалось удалить", http.StatusInternalServerError)
		return
	}

	// После удаления — переходим в родительскую папку
	parent := path.Dir(cleanPath)
	if parent == "." || parent == "/" {
		parent = ""
	}
	http.Redirect(w, r, "/"+parent, http.StatusSeeOther)
}

// Точка входа в программу
func main() {
	log.Println("Инициализация: Создание директории для загрузки")
	os.MkdirAll(uploadDir, os.ModePerm) // Создаёт uploads, если её нет

	// Настраиваем маршруты HTTP
	http.HandleFunc("/", homeHandler)                                                         // Главная страница — список файлов/папок
	http.HandleFunc("/upload", uploadHandler)                                                 // Загрузка файла
	http.HandleFunc("/mkdir", mkdirHandler)                                                   // Создание папки
	http.HandleFunc("/delete/", deleteHandler)                                                // Удаление файла или папки
	http.Handle("/files/", http.StripPrefix("/files/", http.FileServer(http.Dir(uploadDir)))) // Отдача файлов

	log.Println("Сервер запущен на: http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil)) // Запуск HTTP-сервера
}
