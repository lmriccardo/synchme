APP_NAME=app

.PHONY: build run test lint clean

build:
	go build -o bin/$(APP_NAME) ./cmd/app

run:
	go run ./cmd/app

test:
	go test ./... -v

lint:
	golangci-lint run ./...

clean:
	rm -rf bin
