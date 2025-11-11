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
	upgrader   = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	shell      = &Shell{}
	currentDir = getInitialDir()
	dirMu      sync.Mutex
	dirStack   []string

	// use writeMu to serialize writes to websocket to avoid concurrent write panics
	writeMu sync.Mutex
)

type Shell struct {
	mu  sync.Mutex
	cmd *exec.Cmd
}

func getInitialDir() string {
	dir, _ := os.Getwd()
	return dir
}

// safeWrite отправляет байты как BinaryMessage и сериализует записи.
func safeWrite(conn *websocket.Conn, data []byte) error {
	writeMu.Lock()
	defer writeMu.Unlock()
	if conn == nil {
		return fmt.Errorf("conn is nil")
	}
	return conn.WriteMessage(websocket.BinaryMessage, data)
}

func (s *Shell) Run(command string, conn *websocket.Conn) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dirMu.Lock()
	dir := strings.TrimSpace(currentDir)
	dirMu.Unlock()

	cmdTrim := strings.TrimSpace(command)
	cmdLower := strings.ToLower(cmdTrim)

	// === cd, pushd, popd ===
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

	// выполняем команду в заданной директории (без cd в строке)
	fullCmd := fmt.Sprintf(`chcp 65001 >nul && %s`, cmdTrim)
	cmd := exec.Command("cmd", "/c", fullCmd)
	cmd.Dir = dir

	return runCmd(cmd, conn)
}

func runCmd(cmd *exec.Cmd, conn *websocket.Conn) error {
	// лог для диагностики
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

	// отправка байтов из pipe в websocket
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

func sendPrompt(conn *websocket.Conn) {
	dirMu.Lock()
	prompt := getPrompt(currentDir)
	dirMu.Unlock()
	_ = safeWrite(conn, []byte("\r\n"+prompt))
}

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

func handleCD(command, current string) {
	arg := strings.TrimSpace(command[2:])

	// Если пустой — идти в домашнюю директорию
	if arg == "" {
		home, err := os.UserHomeDir()
		if err == nil && home != "" {
			currentDir = home
		}
		return
	}

	// Если просто \ или / — перейти на корень текущего диска
	if arg == `\` || arg == `/` {
		if len(current) >= 1 {
			drive := strings.ToUpper(string(current[0]))
			currentDir = drive + ":\\"
		}
		return
	}

	// Абсолютный путь вида C:\...
	if len(arg) >= 3 && arg[1] == ':' && (arg[2] == '\\' || arg[2] == '/') {
		drive := strings.ToUpper(string(arg[0]))
		path := arg[2:]
		path = strings.TrimPrefix(path, "\\")
		path = strings.TrimPrefix(path, "/")
		currentDir = drive + ":\\" + strings.ReplaceAll(path, "/", "\\")
		currentDir = filepath.Clean(currentDir)
		return
	}

	// Если .. — подняться на уровень
	if arg == ".." || strings.HasPrefix(arg, "..\\") || strings.HasPrefix(arg, "../") {
		currentDir = filepath.Dir(current)
		currentDir = filepath.Clean(currentDir)
		return
	}

	// Если абсолютный путь по мнению Go
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

func handlePushd(command, current string) {
	arg := strings.TrimSpace(command[5:])
	dirMu.Lock()
	dirStack = append(dirStack, currentDir)
	dirMu.Unlock()
	handleCD("cd "+arg, current)
}

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

func getPrompt(dir string) string {
	return strings.ReplaceAll(dir, "/", "\\") + "> "
}

func main() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	tmpl := template.Must(template.ParseFS(embeddedFiles, "console.gohtml"))
	r.SetHTMLTemplate(tmpl)

	r.GET("/", func(c *gin.Context) {
		dirMu.Lock()
		prompt := getPrompt(currentDir)
		dirMu.Unlock()
		c.HTML(200, "console.gohtml", gin.H{"Prompt": prompt})
	})

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

		_ = safeWrite(conn, []byte("\033[36mLocalWebConsole v4 — dir работает!\033[0m\r\n"))
		_ = safeWrite(conn, []byte(prompt))

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				// при ошибке чтения — просто завершаем соединение
				log.Println("ws read error:", err)
				break
			}

			cmd := strings.TrimSpace(string(msg))
			if cmd == "" {
				sendPrompt(conn)
				continue
			}

			if cmd == "exit" {
				_ = safeWrite(conn, []byte("Пока!\r\n"))
				break
			}

			if cmd == "cls" {
				_ = safeWrite(conn, []byte("\033[H\033[2J"))
				continue
			}

			// Запускаем команду параллельно; Shell.Run синхронизирует внутри себя
			go shell.Run(cmd, conn)
		}
	})

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

	log.Println("LocalWebConsole v4 запущена!")
	log.Println("http://localhost:8080")
	if err := r.Run("127.0.0.1:8080"); err != nil {
		log.Fatal(err)
	}
}
