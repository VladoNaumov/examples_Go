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

// GetMessagesHandler HTTP: получить последние N сообщений
func GetMessagesHandler(w http.ResponseWriter, r *http.Request) {
	msgs := chat.GetMessages()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(msgs)
}

// PostMessageHandler HTTP: отправить сообщение через POST (полезно для curl)
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
	http.HandleFunc("/messages", GetMessagesHandler)        // GET
	http.HandleFunc("/message", PostMessageHandler)         // POST
	http.Handle("/", http.FileServer(http.Dir("./static"))) // serve client

	addr := ":8080"
	fmt.Println("Chat server listening on", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
