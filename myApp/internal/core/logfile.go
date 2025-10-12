package core

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
		_ = mainFile.Close()
		return
	}

	globalLogger = &Logger{
		mainFile:  mainFile,
		errorFile: errorFile,
	}

	// Единственный writer, который маршрутизирует по уровням.
	log.SetOutput(newLevelSplitWriter(mainFile, errorFile, os.Stdout))
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("=== New log file initialized ===")

	go cleanupOldLogs("logs", 7)
}

// levelSplitWriter направляет ERROR в errors-*.log, остальное — в обычный лог.
// Всегда дублирует в stdout. Без рекурсивного логирования внутри.
type levelSplitWriter struct {
	mainW io.Writer
	errW  io.Writer
	outW  io.Writer
	mu    sync.Mutex
}

func newLevelSplitWriter(mainW, errW, outW io.Writer) *levelSplitWriter {
	return &levelSplitWriter{mainW: mainW, errW: errW, outW: outW}
}

func (w *levelSplitWriter) Write(p []byte) (int, error) {
	level := detectLevel(p)

	w.mu.Lock()
	defer w.mu.Unlock()

	// Всегда в stdout (не критично, если он nil)
	if w.outW != nil {
		_, _ = w.outW.Write(p)
	}

	switch level {
	case "ERROR":
		if w.errW != nil {
			return w.errW.Write(p)
		}
	default:
		if w.mainW != nil {
			return w.mainW.Write(p)
		}
	}
	// Если оба отсутствуют — "успешная" запись нулём байт.
	return len(p), nil
}

// detectLevel пытается определить уровень из JSON {"level":"ERROR"} или по текстовым индикаторам.
func detectLevel(p []byte) string {
	s := strings.TrimSpace(string(p))

	// Попытка разобрать как JSON лог, который пишет LogError.
	if strings.HasPrefix(s, "{") && strings.Contains(s, `"level"`) {
		var tmp struct {
			Level string `json:"level"`
		}
		if json.Unmarshal([]byte(s), &tmp) == nil && tmp.Level != "" {
			return strings.ToUpper(tmp.Level)
		}
	}

	// Текстовые индикаторы
	ss := strings.ToUpper(s)
	if strings.Contains(ss, `"LEVEL":"ERROR"`) ||
		strings.Contains(ss, " ERROR ") ||
		strings.HasPrefix(ss, "ERROR ") ||
		strings.Contains(ss, "ERROR:") ||
		strings.Contains(ss, " LEVEL=ERROR") ||
		strings.Contains(ss, " LEVEL=ERR") {
		return "ERROR"
	}

	return "INFO"
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
		_ = globalLogger.mainFile.Close()
		_ = globalLogger.errorFile.Close()
	}
}
