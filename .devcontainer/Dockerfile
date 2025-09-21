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
RUN apt-get update && apt-get install -y git nano less vim \ 
    && apt-get clean -y && rm -rf /var/lib/apt/lists/*

# -----------------------------
# Install Go development tools
# -----------------------------
ENV GO111MODULE=on
ENV PATH=$PATH:/go/bin

RUN apt-get update&& export DEBIAN_FRONTEND=noninteractiv \
    && apt-get install -y make\
    && go install github.com/go-delve/delve/cmd/dlv@latest \
    && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest \
    && go install github.com/air-verse/air@latest \
    && go install github.com/golang/mock/mockgen@latest \
    && apt-get clean -y && rm -rf /var/lib/apt/lists/*

# ---------------------------------
# Set non-root user for development
# ---------------------------------
USER vscode

# -----------------------------
# Set working directory
# -----------------------------
WORKDIR /workspace
