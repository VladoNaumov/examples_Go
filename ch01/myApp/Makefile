GOCMD=go

run:
	APP_ENV=dev HTTP_ADDR=:8080 $(GOCMD) run ./cmd/app

tidy:
	$(GOCMD) mod tidy

lint:
	@echo "включим golangci-lint на следующем шаге"

test:
	$(GOCMD) test ./... -count=1
