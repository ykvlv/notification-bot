.PHONY: build run tidy

build:
	GOFLAGS=-trimpath CGO_ENABLED=0 go build -o bin/notification-bot ./cmd/bot

run:
	BOT_TOKEN=$$BOT_TOKEN DB_PATH=./data/notification.db LOG_LEVEL=debug go run ./cmd/bot

tidy:
	go mod tidy
