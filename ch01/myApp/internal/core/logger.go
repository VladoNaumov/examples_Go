package core

//logger.go
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

type Logger struct {
	mainFile  *os.File
	errorFile *os.File
	mu        sync.Mutex
}

var globalLogger *Logger

type LogEntry struct {
	Time   string                 `json:"time"`
	Level  string                 `json:"level"`
	Msg    string                 `json:"msg"`
	Fields map[string]interface{} `json:"fields,omitempty"`
}

func InitDailyLog() {
	if err := os.MkdirAll("logs", 0755); err != nil {
		log.Fatal("Ошибка создания директории logs:", err)
	}

	dateStr := time.Now().Format("02-01-2006")
	mainPath := filepath.Join("logs", dateStr+".log")            // 17-10-2025.log
	errorPath := filepath.Join("logs", "errors-"+dateStr+".log") // errors-17-10-2025.log

	mainFile, err := os.OpenFile(mainPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal("Ошибка открытия основного лог-файла:", err)
	}

	errorFile, err := os.OpenFile(errorPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("Ошибка открытия файла ошибок: %v", err)
		_ = mainFile.Close()
		return
	}

	globalLogger = &Logger{
		mainFile:  mainFile,
		errorFile: errorFile,
	}

	log.SetOutput(newSplitWriter(mainFile, errorFile))
	log.SetFlags(0) // Без стандартных префиксов

	go cleanupOldLogs("logs", 7)
}

type splitWriter struct {
	mainW io.Writer
	errW  io.Writer
	mu    sync.Mutex
}

func newSplitWriter(mainW, errW io.Writer) *splitWriter {
	return &splitWriter{mainW: mainW, errW: errW}
}

func (w *splitWriter) Write(p []byte) (int, error) {
	level := detectLevel(p)

	w.mu.Lock()
	defer w.mu.Unlock()

	if level == "ERROR" {
		if w.errW != nil {
			_, _ = w.errW.Write(p)
		}
	} else {
		if w.mainW != nil {
			_, _ = w.mainW.Write(p)
		}
	}
	return len(p), nil
}

func detectLevel(p []byte) string {
	s := strings.TrimSpace(string(p))

	if strings.HasPrefix(s, "{") && strings.Contains(s, `"level"`) {
		var tmp struct {
			Level string `json:"level"`
		}
		if json.Unmarshal(p, &tmp) == nil && tmp.Level != "" {
			return strings.ToUpper(tmp.Level)
		}
	}

	ss := strings.ToUpper(s)
	if strings.Contains(ss, `"LEVEL":"ERROR"`) || strings.Contains(ss, " ERROR ") {
		return "ERROR"
	}

	return "INFO"
}

func LogInfo(msg string, fields map[string]interface{}) {
	entry := LogEntry{
		Time:  time.Now().UTC().Format(time.RFC3339),
		Level: "INFO",
		Msg:   msg,
	}
	if fields != nil {
		entry.Fields = fields
	}

	data, _ := json.Marshal(entry)
	log.Println(string(data))
}

func LogError(msg string, fields map[string]interface{}) {
	entry := LogEntry{
		Time:  time.Now().UTC().Format(time.RFC3339),
		Level: "ERROR",
		Msg:   msg,
	}
	if fields != nil {
		entry.Fields = fields
	}

	data, _ := json.Marshal(entry)
	log.Println(string(data))
}

func cleanupOldLogs(dir string, days int) {
	files, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("ERROR: Не удалось прочитать %s: %v", dir, err)
		return
	}

	cutoff := time.Now().AddDate(0, 0, -days)

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		info, err := file.Info()
		if err != nil {
			log.Printf("ERROR: Инфо о %s: %v", file.Name(), err)
			continue
		}

		if info.ModTime().Before(cutoff) {
			path := filepath.Join(dir, file.Name())
			if err := os.Remove(path); err != nil {
				log.Printf("ERROR: Удаление %s: %v", path, err)
			}
		}
	}
}

func Close() {
	if globalLogger != nil {
		globalLogger.mu.Lock()
		defer globalLogger.mu.Unlock()

		if err := globalLogger.mainFile.Close(); err != nil {
			log.Printf("ERROR: Закрытие mainFile: %v", err)
		}
		if err := globalLogger.errorFile.Close(); err != nil {
			log.Printf("ERROR: Закрытие errorFile: %v", err)
		}
		globalLogger = nil
	}
}
