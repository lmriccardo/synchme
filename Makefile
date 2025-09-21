BINDIR=bin
CLIENT_NAME=synchme-client
SERVER_NAME=synchme-server
CLIENT_CMD=./cmd/$(CLIENT_NAME)
SERVER_CMD=./cmd/$(SERVER_NAME)

.PHONY: all build client server run-client run-server test lint clean help

all: build

build: client server

client:
	@echo "Building client..."
	go build -o $(BINDIR)/$(CLIENT_NAME) $(CLIENT_CMD)

server:
	@echo "Building server..."
	go build -o $(BINDIR)/$(SERVER_NAME) $(SERVER_CMD)

run-client: client
	$(BINDIR)/$(CLIENT_NAME)

run-server: server
	$(BINDIR)/$(SERVER_NAME)

# Run server in background, then client
demo: server client
	$(BINDIR)/$(SERVER_NAME) & 
	SERVER_PID=$$!; \
	sleep 2; \
	$(BINDIR)/$(CLIENT_NAME); \
	kill $$SERVER_PID

test:
	go test ./... -v -race

lint:
	golangci-lint run ./...

clean:
	rm -rf $(BINDIR)

help:
	@echo "Available targets:"
	@echo "  build       - Build both client and server"
	@echo "  client      - Build client only"
	@echo "  server      - Build server only"
	@echo "  run-client  - Build and run client"
	@echo "  run-server  - Build and run server"
	@echo "  demo        - Run server, then client, then stop server"
	@echo "  test        - Run tests"
	@echo "  lint        - Run linter"
	@echo "  clean       - Remove binaries"