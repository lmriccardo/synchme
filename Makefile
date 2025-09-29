CLIENT_NAME=synchme-client
SERVER_NAME=synchme-server
CLIENT_CMD=./cmd/$(CLIENT_NAME)
SERVER_CMD=./cmd/$(SERVER_NAME)

# Default target OS/ARCH (can override with "make GOOS=windows")
GOOS ?= linux
GOARCH ?= amd64

BINDIR=bin/$(GOOS)

# Add .exe suffix on Windows
ifeq ($(GOOS),windows)
    EXE=.exe
else
    EXE=
endif

PROTO_DIR = ./api
GEN_DIR   = internal/proto
PROTOC	  = protoc

PROTOS := $(shell find $(PROTO_DIR) -name '*.proto')

.PHONY: all proto build client server run-client run-server demo test lint clean help

all: build

proto: $(PROTOS:$(PROTO_DIR)/%.proto=$(GEN_DIR)/%.pb.go)

# Rule for compiling a single proto file
$(GEN_DIR)/%.pb.go: $(PROTO_DIR)/%.proto
	@PROTO_BASENAME=$(basename $(notdir $<)) ; \
	OUT_DIR=$(dir $@)$$PROTO_BASENAME ; \
	echo "Compiling $< ..." ; \
	mkdir -p $$OUT_DIR ; \
	$(PROTOC) --proto_path=$(PROTO_DIR) \
		--go_out=$$OUT_DIR \
		--go_opt=paths=source_relative \
		--go-grpc_opt=paths=source_relative \
		--go-grpc_out=$$OUT_DIR $<

build: client server

client: proto
	@echo "Building client for $(GOOS)/$(GOARCH)..."
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o $(BINDIR)/$(CLIENT_NAME)$(EXE) $(CLIENT_CMD)

server: proto
	@echo "Building server for $(GOOS)/$(GOARCH)..."
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o $(BINDIR)/$(SERVER_NAME)$(EXE) $(SERVER_CMD)

run-client: client
	$(BINDIR)/$(CLIENT_NAME)$(EXE)

run-server: server
	$(BINDIR)/$(SERVER_NAME)$(EXE)

# Run server in background, then client
demo: server client
ifeq ($(GOOS),windows)
	# Windows: use start /B for background process
	start /B $(BINDIR)/$(SERVER_NAME)$(EXE)
	timeout /t 2 > nul
	$(BINDIR)/$(CLIENT_NAME)$(EXE)
else
	# Unix-like
	$(BINDIR)/$(SERVER_NAME)$(EXE) & \
	SERVER_PID=$$!; \
	sleep 2; \
	$(BINDIR)/$(CLIENT_NAME)$(EXE);
endif

test:
	go test ./... -v -race

lint:
	golangci-lint run ./...

clean:
	rm -rf $(BINDIR)

help:
	@echo "Available targets:"
	@echo "  build       - Build both client and server"
	@echo "  proto 		 - Build protobuffer files into Go Code"
	@echo "  client      - Build client only"
	@echo "  server      - Build server only"
	@echo "  run-client  - Build and run client"
	@echo "  run-server  - Build and run server"
	@echo "  demo        - Run server, then client, then stop server"
	@echo "  test        - Run tests"
	@echo "  lint        - Run linter"
	@echo "  clean       - Remove binaries"
