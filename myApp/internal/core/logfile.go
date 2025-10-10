package core

// logfile.go
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

// Logger — глобальный логгер.
type Logger struct {
	mainFile  *os.File
	errorFile *os.File
	mu        sync.Mutex
}

var globalLogger *Logger

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
		mainFile.Close()
		return
	}

	globalLogger = &Logger{
		mainFile:  mainFile,
		errorFile: errorFile,
	}

	logWriter := io.MultiWriter(mainFile, os.Stdout, newErrorSplitter(errorFile))
	log.SetOutput(logWriter)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("=== New log file initialized ===")

	go cleanupOldLogs("logs", 7)
}

// newErrorSplitter пишет строки с "ERROR" в errorFile (OWASP A09).
func newErrorSplitter(errorFile *os.File) io.Writer {
	return writerFunc(func(p []byte) (n int, err error) {
		line := string(p)
		if strings.Contains(line, "ERROR") {
			globalLogger.mu.Lock()
			defer globalLogger.mu.Unlock()
			_, err = errorFile.Write(p)
			if err != nil {
				log.Printf("Failed to write to error log: %v", err)
			}
		}
		return os.Stdout.Write(p)
	})
}

type writerFunc func(p []byte) (n int, err error)

func (f writerFunc) Write(p []byte) (n int, err error) {
	return f(p)
}

// LogError записывает ошибку в JSON (OWASP A09).
func LogError(msg string, fields map[string]interface{}) {
	if globalLogger == nil {
		log.Printf("ERROR: Logger not initialized: %s, fields: %v", msg, fields)
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
		log.Printf("ERROR: Failed to marshal log entry: %v", err)
		return
	}

	log.Println(string(logData))
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
		globalLogger.mainFile.Close()
		globalLogger.errorFile.Close()
	}
}
