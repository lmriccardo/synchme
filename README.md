# Go DevContainer Template

A ready-to-use **Go development environment** with **VS Code DevContainer**, pre-configured for rapid development:

- Go **1.25**
- **Delve debugger** (`dlv`)
- **GolangCI-Lint** for linting
- **Air** for live-reloading
- **MockGen** for generating mocks
- Networking & DB tools (`ping`, `netcat`, `postgresql-client`, etc.)

Perfect for building **CLI tools, APIs, or microservices** in Go with a smooth developer experience.


## 🗂 Project Structure

```
app/
├── .devcontainer/ # VS Code DevContainer files
├── cmd/ # Main applications
├── internal/ # Private application code
├── pkg/ # Public libraries
├── tests/ # Unit and integration tests
├── .gitignore
├── .air.toml # Air live reload configuration
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## ⚡ Getting Started

### 1. Clone the repo

```bash
git clone https://github.com/lmriccardo/go-template.git
mv go-template <ur-app-name>
```

### 2. Open in VS Code

- Install the [*Remote - Containers*](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers) extension
- Open the folder in VS Code
- *Reopen in the container* when prompted

### 3. Build and Run

```bash
make build # or go build -o bin/app ./cmd/app
make run # or ./bin/app
```

## 🧪 Testing and Linting

```
make test
```

- Runs all tests under `./...`
- Uses Go's standard `testing` package

```
make lint
```

Runs `golangci-lint` on all packages

## ⚙️ Makefile Commands

| **Command**      | **Description**                     |
| ------------ | ------------------------------- |
| `make build` | Build the app binary            |
| `make run`   | Run the app                     |
| `make dev`   | Run with live reload using Air  |
| `make test`  | Run all tests                   |
| `make lint`  | Run linters                     |
| `make clean` | Remove build artifacts (`bin/`) |

## 🌟 Using This Template

You can create a new repository from this template directly on GitHub:

1. Go to the template repository page.
2. Click the green “Use this template” button.
3. Fill in your repository name, description, and choose Public or Private.
4. Click “Create repository from template”.
5. Clone your new repo locally
6. Open in VS Code and optionally reopen in the DevContainer