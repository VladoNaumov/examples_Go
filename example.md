**мини-чат (Chat API + WebSocket + простой фронтенд)** на Go (prototype).

---

# Что будет в мини-чате

1. `Message` и `User` — модели.
2. `ChatService` — хранит последние N сообщений, защищён от гонок, позволяет добавлять/читать сообщения.
3. WebSocket-хэндлер — пушит новые сообщения всем подключённым клиентам.
4. HTTP API — получить последние сообщения, отправить сообщение (POST) — для совместимости с curl.
5. Простой HTML/JS клиент, который подключается по WebSocket и отображает чат.
6. Unit-tests для `ChatService`.

---

# Почему WebSocket?

WebSocket — удобен для реального времени: сервер пушит сообщения всем клиентам без опроса.

---

# Полный код (в одном пакете `main` — файлы можно разбить, я даю в одном файле для простоты)

```go
// filename: main.go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ---------- MODELS ----------

type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Message struct {
	ID        string    `json:"id"`
	User      User      `json:"user"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"createdAt"`
}

// ---------- CHAT SERVICE ----------

// ChatService хранит последние `capacity` сообщений и публикует новые подписчикам.
type ChatService struct {
	mu       sync.Mutex
	messages []Message
	capacity int
	// websocket clients
	clients map[*websocket.Conn]bool
	// broadcast channel for new messages
	broadcast chan Message
}

func NewChatService(capacity int) *ChatService {
	cs := &ChatService{
		messages:  make([]Message, 0, capacity),
		capacity:  capacity,
		clients:   make(map[*websocket.Conn]bool),
		broadcast: make(chan Message, 32),
	}
	// Запускаем горутину, которая разошлёт сообщения подключённым клиентам
	go cs.run()
	return cs
}

// run читает из broadcast и отправляет всем подключённым WebSocket клиентам
func (s *ChatService) run() {
	for msg := range s.broadcast {
		s.mu.Lock()
		for c := range s.clients {
			// отправляем асинхронно — чтобы один проблемный клиент не блокировал остальных
			go func(conn *websocket.Conn, m Message) {
				_ = conn.WriteJSON(m) // если ошибка — клиент будет закрыт при Read loop на стороне сервера
			}(c, msg)
		}
		s.mu.Unlock()
	}
}

// AddMessage добавляет сообщение в буфер и публикует его
func (s *ChatService) AddMessage(m Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Добавляем в срез, поддерживаем capacity (FIFO)
	if len(s.messages) >= s.capacity {
		// сдвиг: выбрасываем старое
		s.messages = append(s.messages[1:], m)
	} else {
		s.messages = append(s.messages, m)
	}

	// Отправляем в broadcast (не под мута)
	// Note: send outside lock — but здесь мы уже в блоке lock; чтобы не блокировать run, отправим в отдельной горутине
	go func(mm Message) {
		s.broadcast <- mm
	}(m)
}

// GetMessages возвращает копию текущих сообщений
func (s *ChatService) GetMessages() []Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Message, len(s.messages))
	copy(out, s.messages)
	return out
}

// RegisterClient добавляет WebSocket клиент
func (s *ChatService) RegisterClient(conn *websocket.Conn) {
	s.mu.Lock()
	s.clients[conn] = true
	s.mu.Unlock()
}

// UnregisterClient удаляет WebSocket клиент и закрывает соединение
func (s *ChatService) UnregisterClient(conn *websocket.Conn) {
	s.mu.Lock()
	if _, ok := s.clients[conn]; ok {
		delete(s.clients, conn)
	}
	s.mu.Unlock()
	_ = conn.Close()
}

// ---------- HTTP + WebSocket HANDLERS ----------

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Для простоты: разрешаем все origin. В prod надо проверять.
	CheckOrigin: func(r *http.Request) bool { return true },
}

var chat = NewChatService(100) // храним последние 100 сообщений

// WSHandler — апгрейдит соединение и читает сообщения от клиента.
// Ожидаем, что клиент отправляет JSON вида { "id":"...", "user":{...}, "text":"..." }.
// На сервере мы добавляем CreatedAt, и пушим всем.
func WSHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("ws upgrade:", err)
		return
	}
	chat.RegisterClient(conn)
	defer chat.UnregisterClient(conn)

	// При подключении: отправим последние сообщения
	if err := conn.WriteJSON(map[string]interface{}{
		"kind":     "initial_messages",
		"messages": chat.GetMessages(),
	}); err != nil {
		log.Println("write initial:", err)
		return
	}

	// Читаем сообщения от клиента
	for {
		var incoming Message
		if err := conn.ReadJSON(&incoming); err != nil {
			// Обычно client disconnects — выход
			log.Println("ws read error (client may disconnect):", err)
			return
		}

		// Сформируем сообщение серверной стороны: назначим ID и CreatedAt
		incoming.ID = fmt.Sprintf("%d", time.Now().UnixNano())
		incoming.CreatedAt = time.Now().UTC()

		// Добавляем в сервис (автоматически разошлёт другим)
		chat.AddMessage(incoming)
	}
}

// HTTP: получить последние N сообщений
func GetMessagesHandler(w http.ResponseWriter, r *http.Request) {
	msgs := chat.GetMessages()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(msgs)
}

// HTTP: отправить сообщение через POST (полезно для curl)
// JSON: { "user": { "id":"u1","name":"Vlad" }, "text": "Hello" }
func PostMessageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var in struct {
		User User   `json:"user"`
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if in.Text == "" {
		http.Error(w, "text required", http.StatusBadRequest)
		return
	}
	m := Message{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		User:      in.User,
		Text:      in.Text,
		CreatedAt: time.Now().UTC(),
	}
	chat.AddMessage(m)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(m)
}

func main() {
	http.HandleFunc("/ws", WSHandler)
	http.HandleFunc("/messages", GetMessagesHandler)    // GET
	http.HandleFunc("/message", PostMessageHandler)     // POST
	http.Handle("/", http.FileServer(http.Dir("./static"))) // serve client

	addr := ":8080"
	fmt.Println("Chat server listening on", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
```

---

# Клиент (простая страница) — положи в `static/index.html`

```html
<!doctype html>
<html>
<head>
  <meta charset="utf-8" />
  <title>Mini Chat</title>
  <style>
    body { font-family: Arial; margin: 20px; }
    #messages { border: 1px solid #ccc; padding: 10px; height: 300px; overflow: auto; }
    .msg { margin-bottom: 8px; }
    .meta { font-size: 0.85em; color: #666; }
  </style>
</head>
<body>
  <h3>Mini Chat</h3>

  <div>
    <label>Your name: <input id="name" value="Guest"/></label>
  </div>

  <div id="messages"></div>

  <div style="margin-top:10px;">
    <input id="text" style="width:70%;" placeholder="Type a message..." />
    <button id="send">Send</button>
  </div>

  <script>
    const nameInput = document.getElementById('name')
    const messagesDiv = document.getElementById('messages')
    const textInput = document.getElementById('text')
    const sendBtn = document.getElementById('send')

    const ws = new WebSocket((location.protocol === 'https:' ? 'wss://' : 'ws://') + location.host + '/ws')

    function appendMessage(m) {
      const el = document.createElement('div')
      el.className = 'msg'
      const time = new Date(m.createdAt).toLocaleTimeString()
      el.innerHTML = `<b>${m.user.name}</b>: ${m.text} <div class="meta">${time}</div>`
      messagesDiv.appendChild(el)
      messagesDiv.scrollTop = messagesDiv.scrollHeight
    }

    ws.onopen = () => console.log('ws open')
    ws.onmessage = (evt) => {
      try {
        const data = JSON.parse(evt.data)
        // initial payload has kind initial_messages
        if (data.kind === 'initial_messages') {
          (data.messages || []).forEach(appendMessage)
        } else {
          // single message
          appendMessage(data)
        }
      } catch (e) {
        console.error('ws msg parse', e)
      }
    }
    ws.onclose = () => console.log('ws closed')

    sendBtn.onclick = () => {
      const text = textInput.value.trim()
      const name = nameInput.value.trim() || 'Guest'
      if (!text) return
      const msg = {
        user: { id: name, name },
        text
      }
      ws.send(JSON.stringify(msg))
      textInput.value = ''
    }
  </script>
</body>
</html>
```

---

# Unit-tests для `ChatService` (файл `chat_service_test.go`)

```go
// filename: chat_service_test.go
package main

import (
	"testing"
	"time"
)

func TestAddAndGetMessages(t *testing.T) {
	s := NewChatService(3)
	now := time.Now().UTC()

	s.AddMessage(Message{ID: "1", User: User{ID: "u1", Name: "A"}, Text: "one", CreatedAt: now})
	s.AddMessage(Message{ID: "2", User: User{ID: "u2", Name: "B"}, Text: "two", CreatedAt: now})
	msgs := s.GetMessages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Text != "one" || msgs[1].Text != "two" {
		t.Fatalf("messages content mismatch")
	}
}

func TestCapacity(t *testing.T) {
	s := NewChatService(2)
	now := time.Now().UTC()
	s.AddMessage(Message{ID: "1", Text: "one", CreatedAt: now})
	s.AddMessage(Message{ID: "2", Text: "two", CreatedAt: now})
	s.AddMessage(Message{ID: "3", Text: "three", CreatedAt: now}) // should evict "one"
	msgs := s.GetMessages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Text != "two" || msgs[1].Text != "three" {
		t.Fatalf("expected [two, three], got [%s, %s]", msgs[0].Text, msgs[1].Text)
	}
}
```

---

# Как запустить

1. Инициализируй модуль и подтяни `gorilla/websocket`:

```bash
go mod init example.com/minichat
go get github.com/gorilla/websocket
```

2. Создай папку `static` и положи туда `index.html`.
3. Запусти:

```bash
go run main.go
```

4. Открой браузер `http://localhost:8080/` — увидишь клиент. Открой в нескольких вкладках — увидишь real-time обмен.

5. Отправка через curl (HTTP POST):

```bash
curl -X POST http://localhost:8080/message \
  -H "Content-Type: application/json" \
  -d '{"user":{"id":"cli","name":"CLI"},"text":"Hello from curl"}'
```

---
**мини-задачник (TODO API)** на Go (prototype). 


# Что будет

1. `models` — тип `Todo`.
2. `service` — `TodoService` (Add, Get, List, Update, Delete, Toggle, Clear).
3. HTTP handlers — REST API: `POST /todos`, `GET /todos`, `GET /todos/{id}`, `PUT /todos/{id}`, `DELETE /todos/{id}`, `POST /todos/{id}/toggle`, `POST /todos/clear`.
4. unit-tests для `TodoService`.
5. Примеры `curl`.

---

# Полный код (один файл `main.go`, пакет `main`)

```go
// file: main.go
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ---------- MODELS ----------
type Todo struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body,omitempty"`
	Done      bool      `json:"done"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ---------- SERVICE ----------
type TodoService struct {
	mu      sync.Mutex
	items   map[int64]Todo
	nextID  int64
	limit   int // optional capacity limit; 0 = unlimited
}

// NewTodoService creates a new service
func NewTodoService(limit int) *TodoService {
	return &TodoService{
		items:  make(map[int64]Todo),
		nextID: 1,
		limit:  limit,
	}
}

// Add creates new todo and returns it
func (s *TodoService) Add(title, body string) (Todo, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Todo{}, errors.New("title required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.limit > 0 && len(s.items) >= s.limit {
		return Todo{}, errors.New("limit reached")
	}

	now := time.Now().UTC()
	id := s.nextID
	s.nextID++

	t := Todo{
		ID:        id,
		Title:     title,
		Body:      strings.TrimSpace(body),
		Done:      false,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.items[id] = t
	return t, nil
}

// Get returns todo by id
func (s *TodoService) Get(id int64) (Todo, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.items[id]
	return t, ok
}

// List returns all todos (optionally filtered by done status)
// If doneFilter == nil -> return all. If true -> only done; false -> only not done.
func (s *TodoService) List(doneFilter *bool) []Todo {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Todo, 0, len(s.items))
	for _, t := range s.items {
		if doneFilter != nil {
			if t.Done != *doneFilter {
				continue
			}
		}
		out = append(out, t)
	}
	// optional: sort by CreatedAt ascending
	// sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

// Update modifies title/body (does not change Done unless provided)
func (s *TodoService) Update(id int64, title, body *string) (Todo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.items[id]
	if !ok {
		return Todo{}, errors.New("not found")
	}
	changed := false
	if title != nil {
		nt := strings.TrimSpace(*title)
		if nt == "" {
			return Todo{}, errors.New("title required")
		}
		t.Title = nt
		changed = true
	}
	if body != nil {
		t.Body = strings.TrimSpace(*body)
		changed = true
	}
	if changed {
		t.UpdatedAt = time.Now().UTC()
		s.items[id] = t
	}
	return t, nil
}

// Delete removes a todo
func (s *TodoService) Delete(id int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[id]; !ok {
		return false
	}
	delete(s.items, id)
	return true
}

// Toggle flips Done and updates UpdatedAt
func (s *TodoService) Toggle(id int64) (Todo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.items[id]
	if !ok {
		return Todo{}, errors.New("not found")
	}
	t.Done = !t.Done
	t.UpdatedAt = time.Now().UTC()
	s.items[id] = t
	return t, nil
}

// Clear removes all todos
func (s *TodoService) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = make(map[int64]Todo)
	s.nextID = 1
}
```

(продолжение — HTTP handlers и main)

```go
// ---------- HTTP HANDLERS ----------

var svc = NewTodoService(0) // 0 = no limit

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func parseIDFromPath(r *http.Request) (int64, error) {
	// path expected like /todos/{id} or /todos/{id}/action
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 2 {
		return 0, errors.New("id not found in path")
	}
	idStr := parts[1]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return 0, errors.New("invalid id")
	}
	return id, nil
}

// POST /todos
// body: { "title": "Buy milk", "body": "2 liters" }
func handleCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var in struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	t, err := svc.Add(in.Title, in.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

// GET /todos?done=1|0  (optional)
func handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	q := r.URL.Query().Get("done")
	var filter *bool
	if q != "" {
		if q == "1" || q == "true" {
			v := true
			filter = &v
		} else if q == "0" || q == "false" {
			v := false
			filter = &v
		} else {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid done filter"})
			return
		}
	}
	list := svc.List(filter)
	writeJSON(w, http.StatusOK, list)
}

// GET /todos/{id}
func handleGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	id, err := parseIDFromPath(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	t, ok := svc.Get(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// PUT /todos/{id}
// body: { "title": "new", "body": "..." } (both optional but title if present must not be empty)
func handleUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	id, err := parseIDFromPath(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	var in map[string]*string
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	var title, body *string
	if v, ok := in["title"]; ok {
		title = v
	}
	if v, ok := in["body"]; ok {
		body = v
	}
	t, err := svc.Update(id, title, body)
	if err != nil {
		if err.Error() == "not found" {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// DELETE /todos/{id}
func handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	id, err := parseIDFromPath(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	ok := svc.Delete(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

// POST /todos/{id}/toggle
func handleToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	// extract id from path prefix /todos/{id}/toggle
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid path"})
		return
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	t, err := svc.Toggle(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// POST /todos/clear
func handleClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	svc.Clear()
	writeJSON(w, http.StatusOK, map[string]string{"ok": "cleared"})
}

// ---------- MAIN ----------
func main() {
	http.HandleFunc("/todos", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			handleCreate(w, r)
		case http.MethodGet:
			handleList(w, r)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
	})
	// routes with id
	http.HandleFunc("/todos/", func(w http.ResponseWriter, r *http.Request) {
		// possible paths:
		// /todos/{id}
		// /todos/{id}/toggle
		// /todos/clear  <-- handled earlier if exact
		if strings.HasSuffix(r.URL.Path, "/toggle") {
			handleToggle(w, r)
			return
		}
		// if method DELETE or GET or PUT target /todos/{id}
		switch r.Method {
		case http.MethodGet:
			handleGet(w, r)
		case http.MethodPut:
			handleUpdate(w, r)
		case http.MethodDelete:
			handleDelete(w, r)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
	})
	// clear endpoint
	http.HandleFunc("/todos/clear", handleClear)

	addr := ":8080"
	fmt.Println("TODO API listening on", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
```

---

# Unit-tests (файл `todo_service_test.go`)

```go
// file: todo_service_test.go
package main

import (
	"testing"
)

func TestAddGetListDelete(t *testing.T) {
	s := NewTodoService(0)
	// add
	a, err := s.Add("Task 1", "body 1")
	if err != nil {
		t.Fatal(err)
	}
	if a.ID == 0 {
		t.Fatalf("expected non-zero id")
	}
	// get
	got, ok := s.Get(a.ID)
	if !ok || got.Title != "Task 1" {
		t.Fatalf("get failed")
	}
	// list
	list := s.List(nil)
	if len(list) != 1 {
		t.Fatalf("list expected 1, got %d", len(list))
	}
	// delete
	if !s.Delete(a.ID) {
		t.Fatalf("delete expected true")
	}
	if _, ok = s.Get(a.ID); ok {
		t.Fatalf("should be deleted")
	}
}

func TestUpdateToggleClear(t *testing.T) {
	s := NewTodoService(0)
	a, _ := s.Add("T", "")
	// update
	newTitle := "Updated"
	newBody := "B"
	updated, err := s.Update(a.ID, &newTitle, &newBody)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "Updated" || updated.Body != "B" {
		t.Fatalf("update didn't apply")
	}
	// toggle
	toggled, err := s.Toggle(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if toggled.Done != true {
		t.Fatalf("expected done true")
	}
	// clear
	s.Clear()
	if len(s.List(nil)) != 0 {
		t.Fatalf("expected 0 after clear")
	}
}
```

Запуск тестов:

```bash
go test ./...
```

---

# Примеры curl

1. Создать задачу:

```bash
curl -X POST http://localhost:8080/todos \
 -H "Content-Type: application/json" \
 -d '{"title":"Buy milk","body":"2 liters"}'
```

2. Список (все):

```bash
curl http://localhost:8080/todos
```

3. Список только выполненных:

```bash
curl http://localhost:8080/todos?done=1
```

4. Получить одну:

```bash
curl http://localhost:8080/todos/1
```

5. Обновить (PUT):

```bash
curl -X PUT http://localhost:8080/todos/1 \
 -H "Content-Type: application/json" \
 -d '{"title":"Buy bread"}'
```

6. Переключить done:

```bash
curl -X POST http://localhost:8080/todos/1/toggle
```

7. Удалить:

```bash
curl -X DELETE http://localhost:8080/todos/1
```

8. Очистить все:

```bash
curl -X POST http://localhost:8080/todos/clear
```

---

** 4. Онлайн-магазин с продуктами**. 
Я подготовлю учебный, но практичный проект на Go: 
каталог продуктов (CRUD), корзина (CartService), простая «оформление заказа» (mock checkout), 
HTTP JSON API и тесты. Всё в одном ответе — можно скопировать и запустить.

# Что внутри

1. `ProductService` — CRUD для товаров (in-memory, с предзагрузкой).
2. `CartService` — работа с корзиной (Add/Update/Remove/Clear/Items/Total), `sync.Mutex`.
3. HTTP API:

   * `/products` — GET (список) / POST (создать)
   * `/products/{id}` — GET, PUT, DELETE
   * `/cart/*` — /cart/add, /cart/update, /cart/get, /cart/remove, /cart/clear
   * `/checkout` — POST: имитирует оформление заказа (вернёт snapshot и total)
4. Unit-tests: для ProductService и CartService.
5. Примеры `curl`.

> Замечание: для простоты цен используются `float64`. В реальных проектах лучше использовать decimal (shopspring/decimal) или хранить в cents (int64).

---

# Код (файл `main.go`)

```go
// filename: main.go
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ---------- MODELS ----------
type Product struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Price       float64 `json:"price"` // В проде: decimal или cents
	Stock       int     `json:"stock"` // количество на складе
	CreatedAt   int64   `json:"createdAt"`
	UpdatedAt   int64   `json:"updatedAt"`
}

type CartItem struct {
	Product  Product `json:"product"`
	Quantity int     `json:"quantity"`
}

type Order struct {
	ID        string     `json:"id"`
	Items     []CartItem `json:"items"`
	Total     float64    `json:"total"`
	CreatedAt int64      `json:"createdAt"`
}

// ---------- PRODUCT SERVICE (in-memory) ----------
type ProductService struct {
	mu      sync.Mutex
	items   map[int64]Product
	nextID  int64
}

func NewProductService() *ProductService {
	s := &ProductService{
		items:  make(map[int64]Product),
		nextID: 1,
	}
	// seed with sample products
	s.Create(Product{Name: "Shampoo", Description: "Fresh & clean", Price: 10.5, Stock: 20})
	s.Create(Product{Name: "Conditioner", Description: "Soft hair", Price: 8.0, Stock: 15})
	s.Create(Product{Name: "Mask", Description: "Deep care", Price: 12.0, Stock: 8})
	return s
}

func (s *ProductService) Create(p Product) Product {
	s.mu.Lock()
	defer s.mu.Unlock()
	p.ID = s.nextID
	p.CreatedAt = time.Now().Unix()
	p.UpdatedAt = p.CreatedAt
	s.nextID++
	s.items[p.ID] = p
	return p
}

func (s *ProductService) Get(id int64) (Product, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.items[id]
	return p, ok
}

func (s *ProductService) List() []Product {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Product, 0, len(s.items))
	for _, p := range s.items {
		out = append(out, p)
	}
	return out
}

func (s *ProductService) Update(id int64, patch Product) (Product, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.items[id]
	if !ok {
		return Product{}, errors.New("not found")
	}
	// update fields if non-zero/empty considered changed
	if strings.TrimSpace(patch.Name) != "" {
		cur.Name = patch.Name
	}
	if patch.Description != "" {
		cur.Description = patch.Description
	}
	if patch.Price > 0 {
		cur.Price = patch.Price
	}
	if patch.Stock >= 0 {
		cur.Stock = patch.Stock
	}
	cur.UpdatedAt = time.Now().Unix()
	s.items[id] = cur
	return cur, nil
}

func (s *ProductService) Delete(id int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[id]; !ok {
		return false
	}
	delete(s.items, id)
	return true
}

// ---------- CART SERVICE (per app instance, in-memory) ----------
type CartService struct {
	mu    sync.Mutex
	items map[int64]CartItem // key = product.ID
}

func NewCartService() *CartService {
	return &CartService{
		items: make(map[int64]CartItem),
	}
}

func (c *CartService) Add(p Product, qty int) error {
	if qty <= 0 {
		return errors.New("quantity must be >=1")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if it, ok := c.items[p.ID]; ok {
		it.Quantity += qty
		c.items[p.ID] = it
	} else {
		c.items[p.ID] = CartItem{Product: p, Quantity: qty}
	}
	return nil
}

func (c *CartService) Update(productID int64, qty int) error {
	if qty < 0 {
		return errors.New("quantity must be >=0")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if qty == 0 {
		delete(c.items, productID)
		return nil
	}
	if it, ok := c.items[productID]; ok {
		it.Quantity = qty
		c.items[productID] = it
		return nil
	}
	return errors.New("product not in cart")
}

func (c *CartService) Remove(productID int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, productID)
}

func (c *CartService) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[int64]CartItem)
}

func (c *CartService) Items() []CartItem {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]CartItem, 0, len(c.items))
	for _, it := range c.items {
		out = append(out, it)
	}
	return out
}

func (c *CartService) Total() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	var total float64
	for _, it := range c.items {
		total += float64(it.Quantity) * it.Product.Price
	}
	return total
}

// CreateOrder performs a simple mock checkout:
// - checks stock in productService (reduce stock if enough) and returns order snapshot.
// - if any item stock insufficient -> error and no change.
func CreateOrder(cart *CartService, products *ProductService) (Order, error) {
	// lock both to avoid race: lock productService then cart (consistent order)
	products.mu.Lock()
	cart.mu.Lock()

	// ensure unlock at end
	defer products.mu.Unlock()
	defer cart.mu.Unlock()

	// check stock
	for _, it := range cart.items {
		p, ok := products.items[it.Product.ID]
		if !ok {
			return Order{}, fmt.Errorf("product %d not found", it.Product.ID)
		}
		if p.Stock < it.Quantity {
			return Order{}, fmt.Errorf("not enough stock for product %d", it.Product.ID)
		}
	}

	// reduce stock
	for _, it := range cart.items {
		p := products.items[it.Product.ID]
		p.Stock -= it.Quantity
		p.UpdatedAt = time.Now().Unix()
		products.items[it.Product.ID] = p
	}

	// create order snapshot
	items := make([]CartItem, 0, len(cart.items))
	var total float64
	for _, it := range cart.items {
		items = append(items, it)
		total += float64(it.Quantity) * it.Product.Price
	}
	order := Order{
		ID:        fmt.Sprintf("ord-%d", time.Now().UnixNano()),
		Items:     items,
		Total:     total,
		CreatedAt: time.Now().Unix(),
	}

	// clear cart
	cart.items = make(map[int64]CartItem)

	return order, nil
}

// ---------- HTTP HELPERS ----------
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// parse id from path like /products/{id}
func parseIDFromPath(path string) (int64, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 {
		return 0, errors.New("id not found")
	}
	idStr := parts[1]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return 0, errors.New("invalid id")
	}
	return id, nil
}

// ---------- GLOBAL SERVICES ----------
var products = NewProductService()
var cart = NewCartService()

// ---------- HTTP HANDLERS: Products ----------
func handleProducts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list := products.List()
		writeJSON(w, http.StatusOK, list)
	case http.MethodPost:
		var in Product
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		if strings.TrimSpace(in.Name) == "" || in.Price <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and positive price required"})
			return
		}
		if in.Stock < 0 {
			in.Stock = 0
		}
		created := products.Create(in)
		writeJSON(w, http.StatusCreated, created)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func handleProductByID(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDFromPath(r.URL.Path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	switch r.Method {
	case http.MethodGet:
		p, ok := products.Get(id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusOK, p)
	case http.MethodPut:
		var patch Product
		if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		updated, err := products.Update(id, patch)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, updated)
	case http.MethodDelete:
		ok := products.Delete(id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusNoContent, nil)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

// ---------- HTTP HANDLERS: Cart ----------
type addReq struct {
	ProductID int64 `json:"productId"`
	Quantity  int   `json:"quantity"`
}

func handleCartAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var in addReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	p, ok := products.Get(in.ProductID)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "product not found"})
		return
	}
	if in.Quantity <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "quantity must be >=1"})
		return
	}
	// optional: check stock now (do not reserve; final check on checkout)
	if p.Stock < in.Quantity {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "not enough stock"})
		return
	}
	if err := cart.Add(p, in.Quantity); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, cart.Items())
}

type updateReq struct {
	ProductID int64 `json:"productId"`
	Quantity  int   `json:"quantity"`
}

func handleCartUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var in updateReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	// if increasing qty, check stock
	if in.Quantity > 0 {
		p, ok := products.Get(in.ProductID)
		if !ok {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "product not found"})
			return
		}
		if p.Stock < in.Quantity {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "not enough stock"})
			return
		}
	}
	if err := cart.Update(in.ProductID, in.Quantity); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, cart.Items())
}

func handleCartGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	resp := struct {
		Items []CartItem `json:"items"`
		Total float64    `json:"total"`
	}{
		Items: cart.Items(),
		Total: cart.Total(),
	}
	writeJSON(w, http.StatusOK, resp)
}

func handleCartRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var in struct {
		ProductID int64 `json:"productId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	cart.Remove(in.ProductID)
	writeJSON(w, http.StatusOK, cart.Items())
}

func handleCartClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	cart.Clear()
	writeJSON(w, http.StatusOK, map[string]string{"ok": "cleared"})
}

// ---------- HTTP: Checkout ----------
func handleCheckout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	order, err := CreateOrder(cart, products)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, order)
}

// ---------- MAIN ----------
func main() {
	// product routes
	http.HandleFunc("/products", handleProducts)
	http.HandleFunc("/products/", handleProductByID)

	// cart routes
	http.HandleFunc("/cart/add", handleCartAdd)
	http.HandleFunc("/cart/update", handleCartUpdate)
	http.HandleFunc("/cart/get", handleCartGet)
	http.HandleFunc("/cart/remove", handleCartRemove)
	http.HandleFunc("/cart/clear", handleCartClear)

	// checkout
	http.HandleFunc("/checkout", handleCheckout)

	addr := ":8080"
	fmt.Println("Online shop API listening on", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
```

---

# Примеры `curl`

1. Список продуктов:

```bash
curl http://localhost:8080/products
```

2. Получить продукт:

```bash
curl http://localhost:8080/products/1
```

3. Создать продукт:

```bash
curl -X POST http://localhost:8080/products \
 -H "Content-Type: application/json" \
 -d '{"name":"Soap","description":"Clean","price":3.5,"stock":50}'
```

4. Обновить продукт:

```bash
curl -X PUT http://localhost:8080/products/1 \
 -H "Content-Type: application/json" \
 -d '{"price":11.0,"stock":18}'
```

5. Добавить в корзину:

```bash
curl -X POST http://localhost:8080/cart/add \
 -H "Content-Type: application/json" \
 -d '{"productId":1,"quantity":2}'
```

6. Получить корзину:

```bash
curl http://localhost:8080/cart/get
```

7. Оформить заказ (checkout):

```bash
curl -X POST http://localhost:8080/checkout
```

8. Очистить корзину:

```bash
curl -X POST http://localhost:8080/cart/clear
```

---

# Unit-tests (две базовые проверки)

Создай файл `main_test.go` рядом с `main.go`:

```go
// file: main_test.go
package main

import "testing"

func TestProductCreateGetListDelete(t *testing.T) {
	ps := NewProductService()
	created := ps.Create(Product{Name: "X", Price: 1.5, Stock: 5})
	if created.ID == 0 {
		t.Fatal("expected id")
	}
	got, ok := ps.Get(created.ID)
	if !ok || got.Name != "X" {
		t.Fatal("get failed")
	}
	list := ps.List()
	if len(list) == 0 {
		t.Fatal("expected non-empty list")
	}
	if !ps.Delete(created.ID) {
		t.Fatal("delete failed")
	}
	if _, ok := ps.Get(created.ID); ok {
		t.Fatal("should be deleted")
	}
}

func TestCartAddUpdateCheckout(t *testing.T) {
	ps := NewProductService()
	c := NewCartService()
	// create product with limited stock
	p := ps.Create(Product{Name: "Y", Price: 2.0, Stock: 2})
	if err := c.Add(p, 1); err != nil {
		t.Fatal(err)
	}
	if err := c.Add(p, 1); err != nil {
		t.Fatal(err)
	}
	// now checkout
	order, err := CreateOrder(c, ps)
	if err != nil {
		t.Fatal(err)
	}
	if order.Total != 4.0 {
		t.Fatalf("expected total 4.0 got %v", order.Total)
	}
	// product stock should be reduced to 0
	pp, _ := ps.Get(p.ID)
	if pp.Stock != 0 {
		t.Fatalf("expected stock 0, got %d", pp.Stock)
	}
}
```

Запуск тестов:

```bash
go test ./...
```

---

**сервер авторизации** на Go с использованием токенов. 
будем использовать **JWT (access token)** для доступа к защищённым ресурсам и **refresh token** (серверно-валидируемый UUID),
чтобы безопасно обновлять access token без постоянной авторизации пользователя.


---

# Ключевая идея

* `POST /register` — регистрация (email + password). Пароль хранится хешем (bcrypt).
* `POST /login` — проверка пароля → возвращает `{ access_token, refresh_token }`.
* `POST /refresh` — присылаете `refresh_token`, получаете новый `access_token` (и по желанию новый refresh token).
* `POST /logout` — инвалидирует/удаляет refresh token на сервере.
* Защищённые маршруты используют `Authorization: Bearer <access_token>`.
* Access token: JWT с коротким сроком (напр., 15 минут). Подписываем HMAC SHA256.
* Refresh token: случайный UUID, хранится на сервере вместе с userID и сроком жизни; при использовании проверяется в хранилище.

---

# Безопасность (важно)

1. **Храните секрет (`JWT_SECRET`) в окружении**, не в коде.
2. **Access token короткоживущий** (10–30 мин). Refresh — дольше (дни/недели).
3. **Храните refresh tokens серверно (revocable)**, или применяйте безопасную ротацию.
4. Всегда используйте HTTPS в продакшне.
5. Для продакшна рассмотрите хранение refresh токенов в DB + привязку к user-agent/ip (опционально).
6. Для паролей используйте bcrypt (мы используем cost по умолчанию).

---

# Полный код (один файл `main.go`)

```go
// file: main.go
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// ---------- CONFIG ----------
var (
	// Секрет для JWT — брать из env
	jwtSecret      = []byte(getEnv("JWT_SECRET", "replace-me-with-random-secret"))
	accessTokenTTL = 15 * time.Minute
	refreshTTL     = 7 * 24 * time.Hour
)

// ---------- MODELS ----------
type User struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	PasswordHash []byte `json:"-"` // не отдаём в JSON
	CreatedAt    int64  `json:"createdAt"`
}

type RefreshToken struct {
	Token     string
	UserID    string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// ---------- IN-MEMORY STORE (replaceable with DB) ----------
type AuthStore struct {
	mu            sync.Mutex
	usersByEmail  map[string]User
	usersByID     map[string]User
	refreshTokens map[string]RefreshToken // token -> data
}

func NewAuthStore() *AuthStore {
	return &AuthStore{
		usersByEmail:  make(map[string]User),
		usersByID:     make(map[string]User),
		refreshTokens: make(map[string]RefreshToken),
	}
}

func (s *AuthStore) CreateUser(email, password string) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return User{}, errors.New("email required")
	}
	if _, exists := s.usersByEmail[email]; exists {
		return User{}, errors.New("email already registered")
	}
	// хешируем пароль
	pwHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, err
	}
	u := User{
		ID:           uuid.NewString(),
		Email:        email,
		PasswordHash: pwHash,
		CreatedAt:    time.Now().Unix(),
	}
	s.usersByEmail[email] = u
	s.usersByID[u.ID] = u
	return u, nil
}

func (s *AuthStore) GetUserByEmail(email string) (User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.usersByEmail[strings.ToLower(strings.TrimSpace(email))]
	return u, ok
}

func (s *AuthStore) GetUserByID(id string) (User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.usersByID[id]
	return u, ok
}

func (s *AuthStore) StoreRefreshToken(token string, userID string, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rt := RefreshToken{
		Token:     token,
		UserID:    userID,
		ExpiresAt: time.Now().Add(ttl),
		CreatedAt: time.Now(),
	}
	s.refreshTokens[token] = rt
}

func (s *AuthStore) GetRefreshToken(token string) (RefreshToken, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rt, ok := s.refreshTokens[token]
	return rt, ok
}

func (s *AuthStore) DeleteRefreshToken(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.refreshTokens, token)
}

// OPTIONAL: delete all refresh tokens for a user (logout from all devices)
func (s *AuthStore) DeleteRefreshTokensForUser(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range s.refreshTokens {
		if v.UserID == userID {
			delete(s.refreshTokens, k)
		}
	}
}

// ---------- JWT Helper ----------
type CustomClaims struct {
	UserID string `json:"sub"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

func createAccessToken(user User) (string, error) {
	now := time.Now()
	claims := CustomClaims{
		UserID: user.ID,
		Email:  user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(accessTokenTTL)),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "example-auth-server",
			ID:        uuid.NewString(), // jti
			Subject:   user.ID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// parse and validate token, returning claims
func parseAccessToken(tokenString string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(t *jwt.Token) (interface{}, error) {
		// проверяем метод
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}

// ---------- HTTP HANDLERS ----------
var store = NewAuthStore()

// Request/Response DTOs
type registerReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"` // Bearer
	ExpiresIn    int64  `json:"expires_in"` // seconds
}

func handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var in registerReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(in.Password) == "" || strings.TrimSpace(in.Email) == "" {
		http.Error(w, "email and password required", http.StatusBadRequest)
		return
	}
	user, err := store.CreateUser(in.Email, in.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// create tokens on register — optional. We'll return tokens so user is logged in immediately.
	access, err := createAccessToken(user)
	if err != nil {
		http.Error(w, "could not create access token", http.StatusInternalServerError)
		return
	}
	refresh := uuid.NewString()
	store.StoreRefreshToken(refresh, user.ID, refreshTTL)

	resp := authResp{
		AccessToken:  access,
		RefreshToken: refresh,
		TokenType:    "Bearer",
		ExpiresIn:    int64(accessTokenTTL.Seconds()),
	}
	writeJSON(w, http.StatusCreated, resp)
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var in loginReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	user, ok := store.GetUserByEmail(in.Email)
	if !ok {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	if err := bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(in.Password)); err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	access, err := createAccessToken(user)
	if err != nil {
		http.Error(w, "could not create access token", http.StatusInternalServerError)
		return
	}
	refresh := uuid.NewString()
	store.StoreRefreshToken(refresh, user.ID, refreshTTL)
	resp := authResp{
		AccessToken:  access,
		RefreshToken: refresh,
		TokenType:    "Bearer",
		ExpiresIn:    int64(accessTokenTTL.Seconds()),
	}
	writeJSON(w, http.StatusOK, resp)
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token"`
}

func handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var in refreshReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if in.RefreshToken == "" {
		http.Error(w, "refresh_token required", http.StatusBadRequest)
		return
	}
	rt, ok := store.GetRefreshToken(in.RefreshToken)
	if !ok || time.Now().After(rt.ExpiresAt) {
		http.Error(w, "invalid or expired refresh token", http.StatusUnauthorized)
		return
	}
	user, ok := store.GetUserByID(rt.UserID)
	if !ok {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}
	// Optionally implement rotation: delete old refresh and issue new
	store.DeleteRefreshToken(in.RefreshToken)
	newRefresh := uuid.NewString()
	store.StoreRefreshToken(newRefresh, user.ID, refreshTTL)

	access, err := createAccessToken(user)
	if err != nil {
		http.Error(w, "could not create access token", http.StatusInternalServerError)
		return
	}
	resp := authResp{
		AccessToken:  access,
		RefreshToken: newRefresh,
		TokenType:    "Bearer",
		ExpiresIn:    int64(accessTokenTTL.Seconds()),
	}
	writeJSON(w, http.StatusOK, resp)
}

type logoutReq struct {
	RefreshToken string `json:"refresh_token"`
	AllDevices   bool   `json:"all_devices,omitempty"` // если true — удаляет все токены пользователя
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var in logoutReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if in.AllDevices {
		// если указан access token, можно извлечь userID, но здесь для простоты требуем refresh_token
		if in.RefreshToken == "" {
			http.Error(w, "refresh_token required for all_devices", http.StatusBadRequest)
			return
		}
		rt, ok := store.GetRefreshToken(in.RefreshToken)
		if !ok {
			http.Error(w, "invalid refresh token", http.StatusBadRequest)
			return
		}
		store.DeleteRefreshTokensForUser(rt.UserID)
		writeJSON(w, http.StatusOK, map[string]string{"ok": "logged out from all devices"})
		return
	}
	if in.RefreshToken != "" {
		store.DeleteRefreshToken(in.RefreshToken)
		writeJSON(w, http.StatusOK, map[string]string{"ok": "logged out"})
		return
	}
	http.Error(w, "refresh_token required", http.StatusBadRequest)
}

// Protected route example
func handleProfile(w http.ResponseWriter, r *http.Request) {
	// userID put to context by middleware
	uid, ok := r.Context().Value("user_id").(string)
	if !ok || uid == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	user, ok := store.GetUserByID(uid)
	if !ok {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}
	// return safe user info
	out := struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}{
		ID:    user.ID,
		Email: user.Email,
	}
	writeJSON(w, http.StatusOK, out)
}

// ---------- AUTH MIDDLEWARE ----------
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Expect header: Authorization: Bearer <token>
		h := r.Header.Get("Authorization")
		if h == "" {
			http.Error(w, "authorization header required", http.StatusUnauthorized)
			return
		}
		parts := strings.SplitN(h, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			http.Error(w, "invalid authorization header", http.StatusUnauthorized)
			return
		}
		token := parts[1]
		claims, err := parseAccessToken(token)
		if err != nil {
			http.Error(w, "invalid token: "+err.Error(), http.StatusUnauthorized)
			return
		}
		// Optional: check user exists
		if _, ok := store.GetUserByID(claims.UserID); !ok {
			http.Error(w, "user not found", http.StatusUnauthorized)
			return
		}
		// put user id into context
		ctx := r.Context()
		ctx = contextWithUserID(ctx, claims.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// small helper to avoid importing full context package multiple times
func contextWithUserID(ctx interface{ Value(key interface{}) interface{} }, userID string) (c interface{ Value(key interface{}) interface{} }) {
	// we need a real context.Context; but can't type-assert easily here.
	// Simpler: import context package at top and use it — will do.
	return // placeholder; real implementation below
}

// We'll correct above: add import "context" and implement properly.
```

> **Примечание:** из-за формата ответа в одном блоке я вставил заглушку `contextWithUserID`. Ниже полная и исправленная версия — с корректными импортами и рабочим middleware.

---

# Исправленный и полный `main.go` (рабочая версия)

```go
// file: main.go
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	jwtSecret      = []byte(getEnv("JWT_SECRET", "replace-me-with-random-secret"))
	accessTokenTTL = 15 * time.Minute
	refreshTTL     = 7 * 24 * time.Hour
)

type User struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	PasswordHash []byte `json:"-"`
	CreatedAt    int64  `json:"createdAt"`
}

type RefreshToken struct {
	Token     string
	UserID    string
	ExpiresAt time.Time
	CreatedAt time.Time
}

type AuthStore struct {
	mu            sync.Mutex
	usersByEmail  map[string]User
	usersByID     map[string]User
	refreshTokens map[string]RefreshToken
}

func NewAuthStore() *AuthStore {
	return &AuthStore{
		usersByEmail:  make(map[string]User),
		usersByID:     make(map[string]User),
		refreshTokens: make(map[string]RefreshToken),
	}
}

func (s *AuthStore) CreateUser(email, password string) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return User{}, errors.New("email required")
	}
	if _, exists := s.usersByEmail[email]; exists {
		return User{}, errors.New("email already registered")
	}
	pwHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, err
	}
	u := User{
		ID:           uuid.NewString(),
		Email:        email,
		PasswordHash: pwHash,
		CreatedAt:    time.Now().Unix(),
	}
	s.usersByEmail[email] = u
	s.usersByID[u.ID] = u
	return u, nil
}

func (s *AuthStore) GetUserByEmail(email string) (User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.usersByEmail[strings.ToLower(strings.TrimSpace(email))]
	return u, ok
}

func (s *AuthStore) GetUserByID(id string) (User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.usersByID[id]
	return u, ok
}

func (s *AuthStore) StoreRefreshToken(token string, userID string, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rt := RefreshToken{
		Token:     token,
		UserID:    userID,
		ExpiresAt: time.Now().Add(ttl),
		CreatedAt: time.Now(),
	}
	s.refreshTokens[token] = rt
}

func (s *AuthStore) GetRefreshToken(token string) (RefreshToken, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rt, ok := s.refreshTokens[token]
	return rt, ok
}

func (s *AuthStore) DeleteRefreshToken(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.refreshTokens, token)
}

func (s *AuthStore) DeleteRefreshTokensForUser(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range s.refreshTokens {
		if v.UserID == userID {
			delete(s.refreshTokens, k)
		}
	}
}

type CustomClaims struct {
	UserID string `json:"sub"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

func createAccessToken(user User) (string, error) {
	now := time.Now()
	claims := CustomClaims{
		UserID: user.ID,
		Email:  user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(accessTokenTTL)),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "example-auth-server",
			ID:        uuid.NewString(),
			Subject:   user.ID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func parseAccessToken(tokenString string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}

var store = NewAuthStore()

type registerReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

func handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var in registerReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(in.Password) == "" || strings.TrimSpace(in.Email) == "" {
		http.Error(w, "email and password required", http.StatusBadRequest)
		return
	}
	user, err := store.CreateUser(in.Email, in.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	access, err := createAccessToken(user)
	if err != nil {
		http.Error(w, "could not create access token", http.StatusInternalServerError)
		return
	}
	refresh := uuid.NewString()
	store.StoreRefreshToken(refresh, user.ID, refreshTTL)

	resp := authResp{
		AccessToken:  access,
		RefreshToken: refresh,
		TokenType:    "Bearer",
		ExpiresIn:    int64(accessTokenTTL.Seconds()),
	}
	writeJSON(w, http.StatusCreated, resp)
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var in loginReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	user, ok := store.GetUserByEmail(in.Email)
	if !ok {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	if err := bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(in.Password)); err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	access, err := createAccessToken(user)
	if err != nil {
		http.Error(w, "could not create access token", http.StatusInternalServerError)
		return
	}
	refresh := uuid.NewString()
	store.StoreRefreshToken(refresh, user.ID, refreshTTL)
	resp := authResp{
		AccessToken:  access,
		RefreshToken: refresh,
		TokenType:    "Bearer",
		ExpiresIn:    int64(accessTokenTTL.Seconds()),
	}
	writeJSON(w, http.StatusOK, resp)
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token"`
}

func handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var in refreshReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if in.RefreshToken == "" {
		http.Error(w, "refresh_token required", http.StatusBadRequest)
		return
	}
	rt, ok := store.GetRefreshToken(in.RefreshToken)
	if !ok || time.Now().After(rt.ExpiresAt) {
		http.Error(w, "invalid or expired refresh token", http.StatusUnauthorized)
		return
	}
	user, ok := store.GetUserByID(rt.UserID)
	if !ok {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}
	// rotation
	store.DeleteRefreshToken(in.RefreshToken)
	newRefresh := uuid.NewString()
	store.StoreRefreshToken(newRefresh, user.ID, refreshTTL)

	access, err := createAccessToken(user)
	if err != nil {
		http.Error(w, "could not create access token", http.StatusInternalServerError)
		return
	}
	resp := authResp{
		AccessToken:  access,
		RefreshToken: newRefresh,
		TokenType:    "Bearer",
		ExpiresIn:    int64(accessTokenTTL.Seconds()),
	}
	writeJSON(w, http.StatusOK, resp)
}

type logoutReq struct {
	RefreshToken string `json:"refresh_token"`
	AllDevices   bool   `json:"all_devices,omitempty"`
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var in logoutReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if in.AllDevices {
		if in.RefreshToken == "" {
			http.Error(w, "refresh_token required for all_devices", http.StatusBadRequest)
			return
		}
		rt, ok := store.GetRefreshToken(in.RefreshToken)
		if !ok {
			http.Error(w, "invalid refresh token", http.StatusBadRequest)
			return
		}
		store.DeleteRefreshTokensForUser(rt.UserID)
		writeJSON(w, http.StatusOK, map[string]string{"ok": "logged out from all devices"})
		return
	}
	if in.RefreshToken != "" {
		store.DeleteRefreshToken(in.RefreshToken)
		writeJSON(w, http.StatusOK, map[string]string{"ok": "logged out"})
		return
	}
	http.Error(w, "refresh_token required", http.StatusBadRequest)
}

func handleProfile(w http.ResponseWriter, r *http.Request) {
	uid, ok := r.Context().Value("user_id").(string)
	if !ok || uid == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	user, ok := store.GetUserByID(uid)
	if !ok {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}
	out := struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}{
		ID:    user.ID,
		Email: user.Email,
	}
	writeJSON(w, http.StatusOK, out)
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if h == "" {
			http.Error(w, "authorization header required", http.StatusUnauthorized)
			return
		}
		parts := strings.SplitN(h, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			http.Error(w, "invalid authorization header", http.StatusUnauthorized)
			return
		}
		token := parts[1]
		claims, err := parseAccessToken(token)
		if err != nil {
			http.Error(w, "invalid token: "+err.Error(), http.StatusUnauthorized)
			return
		}
		if _, ok := store.GetUserByID(claims.UserID); !ok {
			http.Error(w, "user not found", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), "user_id", claims.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	http.HandleFunc("/register", handleRegister)
	http.HandleFunc("/login", handleLogin)
	http.HandleFunc("/refresh", handleRefresh)
	http.HandleFunc("/logout", handleLogout)

	// protected
	http.Handle("/profile", authMiddleware(http.HandlerFunc(handleProfile)))

	addr := ":8080"
	fmt.Println("Auth server listening on", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
```

---

# go.mod и зависимости

Создайте `go.mod` в папке проекта и подтяните зависимости:

```bash
go mod init example.com/authserver
go get github.com/golang-jwt/jwt/v5
go get github.com/google/uuid
go get golang.org/x/crypto/bcrypt
```

---

# Примеры curl

1. Регистрация (получаем access + refresh):

```bash
curl -X POST http://localhost:8080/register \
 -H "Content-Type: application/json" \
 -d '{"email":"vlad@example.com","password":"secret123"}'
```

2. Вход:

```bash
curl -X POST http://localhost:8080/login \
 -H "Content-Type: application/json" \
 -d '{"email":"vlad@example.com","password":"secret123"}'
```

3. Доступ к защищённому /profile:

```bash
curl http://localhost:8080/profile \
 -H "Authorization: Bearer <ACCESS_TOKEN>"
```

4. Обновить access через refresh:

```bash
curl -X POST http://localhost:8080/refresh \
 -H "Content-Type: application/json" \
 -d '{"refresh_token":"<REFRESH_TOKEN>"}'
```

5. Logout (удалить refresh token):

```bash
curl -X POST http://localhost:8080/logout \
 -H "Content-Type: application/json" \
 -d '{"refresh_token":"<REFRESH_TOKEN>"}'
```

6. Logout from all devices:

```bash
curl -X POST http://localhost:8080/logout \
 -H "Content-Type: application/json" \
 -d '{"refresh_token":"<REFRESH_TOKEN>", "all_devices": true}'
```


