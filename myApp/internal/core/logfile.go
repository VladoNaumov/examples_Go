package core

//logfile.go
import (
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Logger управляет файлами логов с ротацией
type Logger struct {
	mainFile  *os.File   // Файл для основных логов
	errorFile *os.File   // Файл для ошибок
	mu        sync.Mutex // Синхронизация доступа к файлам
}

var globalLogger *Logger

// InitDailyLog инициализирует ротацию логов, создавая новые файлы на основе текущей даты
func InitDailyLog() {
	// Создаёт директорию для логов, если не существует
	if err := os.MkdirAll("logs", 0755); err != nil {
		log.Printf("Ошибка создания директории логов: %v", err)
		return
	}

	dateStr := time.Now().Format("02-01-2006")
	mainPath := filepath.Join("logs", dateStr+".log")
	errPath := filepath.Join("logs", "errors-"+dateStr+".log")

	// Открывает файл для основных логов
	mainFile, err := os.OpenFile(mainPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("Ошибка открытия основного лог-файла: %v", err)
		return
	}

	// Открывает файл для ошибок
	errorFile, err := os.OpenFile(errPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("Ошибка открытия файла ошибок: %v", err)
		_ = mainFile.Close()
		return
	}

	globalLogger = &Logger{
		mainFile:  mainFile,
		errorFile: errorFile,
	}

	// Настраивает логгер: INFO в консоль+файл, ERROR только в файл
	log.SetOutput(newLevelSplitWriter(mainFile, errorFile, os.Stdout))
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	//log.Println("=== Новый лог-файл инициализирован ===")

	// Запускает очистку старых логов
	go cleanupOldLogs("logs", 7)
}

// levelSplitWriter разделяет логи: ERROR только в файл, INFO в файл+консоль
type levelSplitWriter struct {
	mainW io.Writer  // Основной лог-файл (INFO)
	errW  io.Writer  // Файл ошибок (ERROR)
	outW  io.Writer  // Вывод в stdout (только INFO)
	mu    sync.Mutex // Синхронизация записи
}

// newLevelSplitWriter создаёт writer с разделением по уровням
func newLevelSplitWriter(mainW, errW, outW io.Writer) *levelSplitWriter {
	return &levelSplitWriter{mainW: mainW, errW: errW, outW: outW}
}

// Write записывает лог: ERROR только в файл, INFO в файл+консоль
func (w *levelSplitWriter) Write(p []byte) (int, error) {
	level := detectLevel(p)

	w.mu.Lock()
	defer w.mu.Unlock()

	// ✅ INFO в консоль (ERROR - НИКОГДА в консоль)
	if level == "INFO" && w.outW != nil {
		_, _ = w.outW.Write(p)
	}

	// Запись в файлы по уровням
	switch level {
	case "ERROR":
		if w.errW != nil {
			_, _ = w.errW.Write(p) // ERROR только в errors-*.log
			return len(p), nil
		}
	default: // INFO и остальное
		if w.mainW != nil {
			_, _ = w.mainW.Write(p) // INFO в основной лог
			return len(p), nil
		}
	}

	return len(p), nil
}

// detectLevel определяет уровень лога из сообщения (JSON или текст)
func detectLevel(p []byte) string {
	s := strings.TrimSpace(string(p))

	// Проверяет JSON-формат лога (LogError, LogInfo)
	if strings.HasPrefix(s, "{") && strings.Contains(s, `"level"`) {
		var tmp struct {
			Level string `json:"level"`
		}
		if json.Unmarshal([]byte(s), &tmp) == nil && tmp.Level != "" {
			return strings.ToUpper(tmp.Level)
		}
	}

	// Проверяет текстовые индикаторы уровня
	ss := strings.ToUpper(s)
	if strings.Contains(ss, `"LEVEL":"ERROR"`) ||
		strings.Contains(ss, " ERROR ") ||
		strings.HasPrefix(ss, "ERROR ") ||
		strings.Contains(ss, "ERROR:") ||
		strings.HasPrefix(ss, "ERROR:") ||
		strings.Contains(ss, " LEVEL=ERROR") ||
		strings.Contains(ss, " LEVEL=ERR") {
		return "ERROR"
	}

	return "INFO"
}

// LogError записывает сообщение об ошибке в формате JSON (только в файл)
func LogError(msg string, fields map[string]interface{}) {
	if globalLogger == nil {
		log.Printf("ERROR: Логгер не инициализирован: %s", msg) // Fallback
		return
	}

	logEntry := map[string]interface{}{
		"level":  "ERROR",
		"msg":    msg,
		"time":   time.Now().UTC().Format(time.RFC3339),
		"fields": fields,
	}
	logData, err := json.Marshal(logEntry)
	if err != nil {
		log.Printf("ERROR: Ошибка сериализации ERROR лога: %v", err)
		return
	}

	log.Println(string(logData)) // detectLevel определит ERROR → только файл
}

// LogInfo записывает информационное сообщение в формате JSON (файл+консоль)
func LogInfo(msg string, fields map[string]interface{}) {
	if globalLogger == nil {
		log.Printf("INFO: %s", msg) // Fallback
		return
	}

	logEntry := map[string]interface{}{
		"level":  "INFO",
		"msg":    msg,
		"time":   time.Now().UTC().Format(time.RFC3339),
		"fields": fields,
	}

	logData, err := json.Marshal(logEntry)
	if err != nil {
		log.Printf("ERROR: Ошибка сериализации INFO лога: %v", err)
		return
	}

	log.Println(string(logData)) // detectLevel определит INFO → файл+консоль
}

// cleanupOldLogs удаляет лог-файлы старше 7 дней
func cleanupOldLogs(dir string, maxAgeDays int) {
	files, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("ERROR: Ошибка чтения директории логов: %v", err)
		return
	}

	cutoff := time.Now().AddDate(0, 0, -maxAgeDays)
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		info, err := file.Info()
		if err != nil {
			log.Printf("ERROR: Ошибка получения информации о файле %s: %v", file.Name(), err)
			continue
		}
		if info.ModTime().Before(cutoff) {
			path := filepath.Join(dir, file.Name())
			if err := os.Remove(path); err != nil {
				log.Printf("ERROR: Ошибка удаления старого лога %s: %v", path, err)
			}
		}
	}
}

// Close закрывает файлы логов при завершении приложения
func Close() {
	if globalLogger != nil {
		globalLogger.mu.Lock()
		defer globalLogger.mu.Unlock()
		_ = globalLogger.mainFile.Close()
		_ = globalLogger.errorFile.Close()
		globalLogger = nil
	}
}
