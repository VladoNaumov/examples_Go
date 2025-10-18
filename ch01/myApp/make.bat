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
    echo Running golangci-lint...

    if not exist "%USERPROFILE%\go\bin\golangci-lint.exe" (
        echo golangci-lint not found. Installing...
        go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
    )

    if exist "%USERPROFILE%\go\bin\golangci-lint.exe" (
        "%USERPROFILE%\go\bin\golangci-lint.exe" run
        if errorlevel 1 (
            echo Lint errors found!
            exit /b 1
        )
        echo Lint check completed successfully.
        exit /b 0
    ) else (
        echo golangci-lint still not found. Add %USERPROFILE%\go\bin to PATH and retry.
        exit /b 1
    )
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