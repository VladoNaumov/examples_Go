package main

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

//go:embed console.gohtml
var embeddedFiles embed.FS

var (
	// Настройки для WebSocket. CheckOrigin = true позволяет подключение с любого источника.
	upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

	// Объект консоли (обёртка над exec.Cmd)
	shell = &Shell{}

	// Текущая рабочая директория (по умолчанию — каталог запуска программы)
	currentDir = getInitialDir()

	// Мьютексы для синхронизации доступа к общей информации
	dirMu    sync.Mutex // защищает currentDir и dirStack
	dirStack []string   // стек каталогов для pushd/popd

	writeMu sync.Mutex // синхронизация записи в WebSocket (нельзя писать из разных горутин)
)

// Структура для управления одной активной командой
type Shell struct {
	mu  sync.Mutex
	cmd *exec.Cmd
}

// Возвращает текущую рабочую директорию при старте программы
func getInitialDir() string {
	dir, _ := os.Getwd()
	return dir
}

// safeWrite — безопасная запись в WebSocket (с блокировкой, чтобы не было одновременной записи)
func safeWrite(conn *websocket.Conn, data []byte) error {
	writeMu.Lock()
	defer writeMu.Unlock()
	if conn == nil {
		return fmt.Errorf("conn is nil")
	}
	return conn.WriteMessage(websocket.BinaryMessage, data)
}

// Run — основной метод, выполняющий введённую пользователем команду
func (s *Shell) Run(command string, conn *websocket.Conn) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dirMu.Lock()
	dir := strings.TrimSpace(currentDir)
	dirMu.Unlock()

	cmdTrim := strings.TrimSpace(command)
	cmdLower := strings.ToLower(cmdTrim)

	// Обработка встроенных команд: cd, pushd, popd
	if isCDCommand(cmdTrim) {
		handleCD(cmdTrim, dir)
		sendPrompt(conn)
		return nil
	}
	if strings.HasPrefix(cmdLower, "pushd ") {
		handlePushd(cmdTrim, dir)
		sendPrompt(conn)
		return nil
	}
	if cmdLower == "popd" {
		handlePopd(conn)
		sendPrompt(conn)
		return nil
	}

	// Для обычных системных команд (выполняются через cmd.exe)
	fullCmd := fmt.Sprintf(`chcp 65001 >nul && %s`, cmdTrim)
	cmd := exec.Command("cmd", "/c", fullCmd)
	cmd.Dir = dir

	return runCmd(cmd, conn)
}

// runCmd — выполняет команду и пересылает stdout/stderr пользователю через WebSocket
func runCmd(cmd *exec.Cmd, conn *websocket.Conn) error {
	log.Printf("exec: %v, dir=%q\n", cmd.Args, cmd.Dir)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		_ = safeWrite(conn, []byte("Ошибка: не удалось получить stdout: "+err.Error()+"\r\n"))
		return err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		_ = safeWrite(conn, []byte("Ошибка: не удалось получить stderr: "+err.Error()+"\r\n"))
		return err
	}

	if err := cmd.Start(); err != nil {
		_ = safeWrite(conn, []byte("Ошибка: "+err.Error()+"\r\n"))
		return err
	}

	// Асинхронно пересылаем данные stdout и stderr в браузер
	sendFromPipe := func(r io.Reader) {
		buf := make([]byte, 4096)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				_ = safeWrite(conn, buf[:n])
			}
			if err != nil {
				if err != io.EOF {
					_ = safeWrite(conn, []byte("pipe read error: "+err.Error()+"\r\n"))
				}
				break
			}
		}
	}

	go sendFromPipe(stdoutPipe)
	go sendFromPipe(stderrPipe)

	err = cmd.Wait()
	sendPrompt(conn)
	if err != nil {
		exitCode := 0
		if cmd.ProcessState != nil {
			exitCode = cmd.ProcessState.ExitCode()
		}
		_ = safeWrite(conn, []byte("exit status "+fmt.Sprint(exitCode)+"\r\n"))
	}
	return nil
}

// sendPrompt — выводит строку приглашения (например: C:\Users\Vladimir>)
func sendPrompt(conn *websocket.Conn) {
	dirMu.Lock()
	prompt := getPrompt(currentDir)
	dirMu.Unlock()
	_ = safeWrite(conn, []byte("\r\n"+prompt))
}

// isCDCommand — определяет, является ли команда командой `cd`
func isCDCommand(cmd string) bool {
	cmdLower := strings.ToLower(strings.TrimSpace(cmd))
	if !strings.HasPrefix(cmdLower, "cd") {
		return false
	}
	if len(cmdLower) == 2 {
		return true
	}
	rest := cmdLower[2:]
	return rest == "" || strings.HasPrefix(rest, " ") || strings.HasPrefix(rest, "\\") || strings.HasPrefix(rest, "/") || strings.Contains(rest, ":")
}

// handleCD — реализация логики команды `cd`
func handleCD(command, current string) {
	arg := strings.TrimSpace(command[2:])

	// cd → переход в домашнюю директорию
	if arg == "" {
		home, err := os.UserHomeDir()
		if err == nil && home != "" {
			currentDir = home
		}
		return
	}

	// cd \ → переход в корень текущего диска
	if arg == `\` || arg == `/` {
		if len(current) >= 1 {
			drive := strings.ToUpper(string(current[0]))
			currentDir = drive + ":\\" // пример: C:\
		}
		return
	}

	// Абсолютный путь C:\...
	if len(arg) >= 3 && arg[1] == ':' && (arg[2] == '\\' || arg[2] == '/') {
		drive := strings.ToUpper(string(arg[0]))
		path := arg[2:]
		path = strings.TrimPrefix(path, "\\")
		path = strings.TrimPrefix(path, "/")
		currentDir = drive + ":\\" + strings.ReplaceAll(path, "/", "\\")
		currentDir = filepath.Clean(currentDir)
		return
	}

	// cd .. — переход на уровень выше
	if arg == ".." || strings.HasPrefix(arg, "..\\") || strings.HasPrefix(arg, "../") {
		currentDir = filepath.Dir(current)
		currentDir = filepath.Clean(currentDir)
		return
	}

	// Иначе — относительный путь
	if filepath.IsAbs(arg) {
		currentDir = filepath.Clean(arg)
	} else {
		currentDir = filepath.Join(current, arg)
	}

	currentDir = filepath.Clean(currentDir)
	if abs, err := filepath.Abs(currentDir); err == nil {
		currentDir = abs
	}
}

// handlePushd — сохраняет текущую директорию в стек и выполняет cd
func handlePushd(command, current string) {
	arg := strings.TrimSpace(command[5:])
	dirMu.Lock()
	dirStack = append(dirStack, currentDir)
	dirMu.Unlock()
	handleCD("cd "+arg, current)
}

// handlePopd — возвращает предыдущую директорию из стека
func handlePopd(conn *websocket.Conn) {
	dirMu.Lock()
	defer dirMu.Unlock()
	if len(dirStack) == 0 {
		_ = safeWrite(conn, []byte("Стек пуст.\r\n"))
		return
	}
	currentDir = dirStack[len(dirStack)-1]
	dirStack = dirStack[:len(dirStack)-1]
	_ = safeWrite(conn, []byte("Перешёл в: "+currentDir+"\r\n"))
}

// getPrompt — возвращает строку приглашения, например: "C:\Projects>"
func getPrompt(dir string) string {
	return strings.ReplaceAll(dir, "/", "\\") + "> "
}

func main() {
	// Настройка Gin (без лишних логов)
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// Загружаем встроенный HTML-шаблон консоли
	tmpl := template.Must(template.ParseFS(embeddedFiles, "console.gohtml"))
	r.SetHTMLTemplate(tmpl)

	// Основная страница — отображает HTML с консолью
	r.GET("/", func(c *gin.Context) {
		dirMu.Lock()
		prompt := getPrompt(currentDir)
		dirMu.Unlock()
		c.HTML(200, "console.gohtml", gin.H{"Prompt": prompt})
	})

	// WebSocket — взаимодействие с консолью
	r.GET("/ws", func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Println("ws upgrade error:", err)
			return
		}
		defer conn.Close()

		dirMu.Lock()
		prompt := getPrompt(currentDir)
		dirMu.Unlock()

		_ = safeWrite(conn, []byte("\033[36mLocalWebConsole v4 — готова к работе!\033[0m\r\n"))
		_ = safeWrite(conn, []byte(prompt))

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				log.Println("ws read error:", err)
				break
			}

			cmd := strings.TrimSpace(string(msg))
			if cmd == "" {
				sendPrompt(conn)
				continue
			}

			switch cmd {
			case "exit":
				_ = safeWrite(conn, []byte("Пока!\r\n"))
				return
			case "cls":
				_ = safeWrite(conn, []byte("\033[H\033[2J"))
				continue
			}

			// Команды выполняются в отдельных горутинах
			go shell.Run(cmd, conn)
		}
	})

	// Маршрут для автодополнения имён файлов/папок
	r.POST("/complete", func(c *gin.Context) {
		var req struct{ Prefix string }
		if err := c.BindJSON(&req); err != nil {
			c.JSON(400, nil)
			return
		}

		dirMu.Lock()
		dir := currentDir
		dirMu.Unlock()

		matches := []string{}
		entries, err := os.ReadDir(dir)
		if err != nil {
			c.JSON(200, matches)
			return
		}

		for _, e := range entries {
			name := e.Name()
			if strings.HasPrefix(strings.ToLower(name), strings.ToLower(req.Prefix)) {
				if e.IsDir() {
					name += "\\"
				}
				matches = append(matches, name)
			}
		}
		c.JSON(200, matches)
	})

	// Запуск сервера
	log.Println("LocalWebConsole v4 запущена!")
	log.Println("Открой: http://localhost:8080")
	if err := r.Run("127.0.0.1:8080"); err != nil {
		log.Fatal(err)
	}
}
