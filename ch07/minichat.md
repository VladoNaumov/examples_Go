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
