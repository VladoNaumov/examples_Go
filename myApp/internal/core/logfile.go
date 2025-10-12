// internal/core/logfile.go
package core

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Logger — глобальный логгер.
type Logger struct {
	mainFile  *os.File
	errorFile *os.File
	mu        sync.Mutex
}

var globalLogger *Logger

// levelWriter — пишет запись ровно в один файл по уровню.
type levelWriter struct {
	lg *Logger
}

func (w *levelWriter) Write(p []byte) (n int, err error) {
	// Пытаемся извлечь уровень из JSON (если есть).
	var tmp struct {
		Level string `json:"level"`
	}
	level := ""
	if err := json.Unmarshal(bytes.TrimSpace(p), &tmp); err == nil && tmp.Level != "" {
		level = strings.ToUpper(tmp.Level)
	} else {
		s := strings.ToUpper(string(p))
		if strings.Contains(s, `"LEVEL":"ERROR"`) || strings.Contains(s, " ERROR ") {
			level = "ERROR"
		}
	}

	globalLogger.mu.Lock()
	defer globalLogger.mu.Unlock()

	if level == "ERROR" && globalLogger.errorFile != nil {
		return globalLogger.errorFile.Write(p)
	}

	if globalLogger.mainFile != nil {
		return globalLogger.mainFile.Write(p)
	}

	// fallback — только stdout
	return len(p), nil
}

// InitDailyLog инициализирует лог-файлы с ротацией (OWASP A09).
func InitDailyLog() {
	if err := os.MkdirAll("logs", 0755); err != nil {
		log.Printf("Failed to create logs directory: %v", err)
		return
	}

	dateStr := time.Now().Format("02-01-2006")
	mainPath := filepath.Join("logs", dateStr+".log")
	errPath := filepath.Join("logs", "errors-"+dateStr+".log")

	mainFile, err := os.OpenFile(mainPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("Failed to open main log file: %v", err)
		return
	}

	errorFile, err := os.OpenFile(errPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("Failed to open error log file: %v", err)
		_ = mainFile.Close()
		return
	}

	globalLogger = &Logger{
		mainFile:  mainFile,
		errorFile: errorFile,
	}

	// Консоль + маршрутизатор по уровню.
	logWriter := io.MultiWriter(os.Stdout, &levelWriter{lg: globalLogger})
	log.SetOutput(logWriter)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("=== New log file initialized ===")

	go cleanupOldLogs("logs", 7)
}

// ---------- Публичные функции ----------

// LogError записывает ошибку в JSON (OWASP A09).
func LogError(msg string, fields map[string]interface{}) {
	logWithLevel("ERROR", msg, fields)
}

// LogInfo — обычная информационная запись.
func LogInfo(msg string, fields map[string]interface{}) {
	logWithLevel("INFO", msg, fields)
}

// Общая функция записи.
func logWithLevel(level, msg string, fields map[string]interface{}) {
	if globalLogger == nil {
		log.Printf("%s: %s, fields: %v", strings.ToUpper(level), msg, fields)
		return
	}

	entry := map[string]interface{}{
		"level":  strings.ToUpper(level),
		"msg":    msg,
		"time":   time.Now().UTC().Format(time.RFC3339),
		"fields": fields,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		log.Printf("ERROR: Failed to marshal log entry: %v", err)
		return
	}

	log.Println(string(data))
}

// cleanupOldLogs удаляет старые логи (OWASP A09).
func cleanupOldLogs(dir string, maxAgeDays int) {
	files, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("ERROR: Failed to read logs directory: %v", err)
		return
	}

	cutoff := time.Now().AddDate(0, 0, -maxAgeDays)
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		info, err := file.Info()
		if err != nil {
			log.Printf("ERROR: Failed to get file info for %s: %v", file.Name(), err)
			continue
		}
		if info.ModTime().Before(cutoff) {
			path := filepath.Join(dir, file.Name())
			if err := os.Remove(path); err != nil {
				log.Printf("ERROR: Failed to remove old log %s: %v", path, err)
			}
		}
	}
}

// Close закрывает файлы логов (OWASP A09).
func Close() {
	if globalLogger != nil {
		globalLogger.mu.Lock()
		defer globalLogger.mu.Unlock()
		if globalLogger.mainFile != nil {
			_ = globalLogger.mainFile.Close()
		}
		if globalLogger.errorFile != nil {
			_ = globalLogger.errorFile.Close()
		}
	}
}
