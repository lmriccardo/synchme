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