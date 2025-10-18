
### 
- ch00  "Go json-srv **Проект JSON API Server ( Go 1.25.1 )"
- ch01  "каркас интернет магазина ( Go 1.25.1 )"





-INFO:

###  Инициализируй модуль Go

Командой:

```bash
go mod init myapp
```

* `myapp` — это имя твоего модуля (можно указать и путь, например `github.com/vladimir/myapp` если планируешь заливать на GitHub).
* В результате создаётся файл `go.mod`, где хранится имя модуля и версия Go.

Пример `go.mod`:

```go
module myapp

go 1.23
```

---

### 2. Создай файл `main.go`

Создай в корне проекта файл:

```bash
nano main.go
```

И напиши туда минимальный код:

```go
package main

import "fmt"

func main() {
    fmt.Println("Hello, Go!")
}
```
### Явная установка пакета
```
go get github.com/gorilla/mux
```
---

### 3. Запусти проект

Выполни:

```bash
go run .
```
или (эквивалентно)

```bash
go run main.go
```

Результат:

```
Hello, Go!
```

---

### 4. (Опционально) Собери бинарник

Если хочешь получить исполняемый файл:

```bash
go build
```

После этого в папке появится файл `myapp.exe` (в Windows) или просто `myapp` (в Linux/Mac).



