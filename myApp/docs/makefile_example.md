
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

```

## ⚙️ Теперь доступно:

| Команда        | Описание                                           |
| -------------- | -------------------------------------------------- |
| `.\make run`   | запустить проект                                   |
| `.\make build` | собрать бинарник `bin\app.exe`                     |
| `.\make start` | запустить бинарник                                 |
| `.\make clean` | удалить `bin`                                      |
| `.\make test`  | запустить все Go-тесты                             |
| `.\make lint`  | форматировать и проверить код (`go fmt`, `go vet`) |

---