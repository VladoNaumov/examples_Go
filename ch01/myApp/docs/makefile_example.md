
### 📄 `Makefile`

```Make

GOCMD=go

run:
APP_ENV=dev HTTP_ADDR=:8080 $(GOCMD) run ./cmd/app


tidy:
$(GOCMD) mod tidy

lint:
@echo "включим golangci-lint на следующем шаге"

test:
$(GOCMD) test ./... -count=1

```


### 📄 `make.bat`

```bat

@echo off
setlocal enabledelayedexpansion

REM === Настройки окружения ===
set APP_ENV=dev
set HTTP_ADDR=:8080

if "%1"=="" (
    echo Usage: make [run ^| build ^| start ^| clean ^| test ^| lint ^| tidy]
    exit /b 0
)

if "%1"=="run" (
    echo Running app on %HTTP_ADDR%...
    go run ./cmd/app
    exit /b
)

if "%1"=="build" (
    echo Building binary...
    if not exist bin mkdir bin
    go build -o bin\app.exe ./cmd/app
    echo Build complete: bin\app.exe
    exit /b
)

if "%1"=="start" (
    echo Starting binary...
    if not exist bin\app.exe (
        echo Binary not found. Run: make build
        exit /b 1
    )
    bin\app.exe
    exit /b
)

if "%1"=="clean" (
    echo Cleaning build files...
    if exist bin rmdir /s /q bin
    echo Clean done.
    exit /b
)

if "%1"=="test" (
    echo Running Go tests...
    go test ./... -v
    exit /b
)

if "%1"=="lint" (
    echo  Running Go fmt and vet...
    go fmt ./...
    go vet ./...
    echo Lint check completed.
    exit /b
)

if "%1"=="tidy" (
    echo  Running go mod tidy...
    go mod tidy
    echo Dependencies updated.
    exit /b
)

echo Unknown command: %1
echo Usage: make [run ^| build ^| start ^| clean ^| test ^| lint ^| tidy]
exit /b 1



## 🚀 Запуск
- **NGINX**: Настроить `nginx.conf` (TLS, rate limiting, кэш, gzip).
- **Go**: `go run ./cmd/app`.

| Команда       | Описание                          |
|---------------|-----------------------------------|
| `make run`    | Запустить проект                  |
| `make build`  | Собрать bin/app.exe               |
| `make start`  | Запустить бинарник                |
| `make clean`  | Удалить bin                       |
| `make test`   | Запустить тесты                   |
| `make lint`   | go fmt, go vet                    |

### Как запускать (cmd / PowerShell)
Перейди в папку проекта и запускай:

| Команда (cmd / PowerShell)          | Назначение                          | Примечание                                                |
| ----------------------------------- | ----------------------------------- | --------------------------------------------------------- |
| `.\make.bat run` <br>или `make run` | **Запуск приложения из исходников** | Запускает `go run ./cmd/app` на порту `:8080`             |
| `.\make.bat build`                  | **Сборка бинарника**                | Создаёт `bin\app.exe`                                     |
| `.\make.bat start`                  | **Запуск собранного бинарника**     | Запускает `bin\app.exe`, если он существует               |
| `.\make.bat clean`                  | **Очистка сборки**                  | Удаляет папку `bin`                                       |
| `.\make.bat test`                   | **Запуск тестов Go**                | Выполняет `go test ./... -v`                              |
| `.\make.bat lint`                   | **Проверка кода линтером**          | Запускает `golangci-lint`; если не найден — установит его |
| `.\make.bat tidy`                   | **Обновление зависимостей**         | Выполняет `go mod tidy`                                   |

```

---