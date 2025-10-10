FROM mcr.microsoft.com/devcontainers/go:1.25

# -----------------------------
# Install networking facilities
# -----------------------------
RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && apt-get install -y \
    net-tools \
    iputils-ping \
    dnsutils \
    netcat-traditional \
    socat \
    tcpdump \
    traceroute \
    wget \
    && apt-get clean -y && rm -rf /var/lib/apt/lists/*

# -----------------------------------------------------
# Install Git & source control & some convenience tools
# -----------------------------------------------------
RUN apt-get update && apt-get install -y git nano less vim make \ 
    && apt-get clean -y && rm -rf /var/lib/apt/lists/*

# ---------------------------------
# Set non-root user BEFORE installing Go tools
# ---------------------------------
USER vscode

# -----------------------------
# Set up Go environment for vscode user
# -----------------------------
ENV GO111MODULE=on
ENV GOPATH=/home/vscode/go
ENV GOMODCACHE=/home/vscode/go/pkg/mod
ENV PATH=$PATH:$GOPATH/bin
ENV CGO_ENABLED=1

# Create Go directories with proper ownership
RUN mkdir -p $GOPATH/bin $GOPATH/pkg $GOPATH/src

# -----------------------------
# Install Go development tools as vscode user
# -----------------------------
RUN go install github.com/go-delve/delve/cmd/dlv@latest \
    && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest \
    && go install github.com/air-verse/air@latest \
    && go install github.com/golang/mock/mockgen@latest

RUN go mod download github.com/fsnotify/fsnotify@v1.9.0
RUN go mod download github.com/sergi/go-diff@v1.4.0
RUN go mod download github.com/BurntSushi/toml@v1.5.0
RUN go mod download github.com/go-playground/validator/v10@v10.27.0
RUN go mod download github.com/radovskyb/watcher@v1.0.7

# Install the protocol buffer Go plugin
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Install the protocol buffer compiler
RUN PB_REL="https://github.com/protocolbuffers/protobuf/releases" \
    && curl -L -o /tmp/protoc.zip $PB_REL/download/v30.2/protoc-30.2-linux-x86_64.zip \
    && unzip /tmp/protoc.zip -d $HOME/.local \
    && rm /tmp/protoc.zip

RUN go mod download github.com/google/uuid@v1.6.0
RUN go mod download github.com/mattn/go-sqlite3@v1.14.32
RUN go mod download github.com/joho/godotenv@v1.5.1