.PHONY: all proto build clean test run dev install

# 变量定义
BINARY_NAME=serverprobe-agent
PROTO_DIR=../proto
PROTO_OUT_DIR=./proto
GO_FILES=$(shell find . -name '*.go' -type f)

all: proto build

# 生成 Proto 代码
proto:
	@echo "Generating proto files..."
	@mkdir -p $(PROTO_OUT_DIR)
	protoc --go_out=$(PROTO_OUT_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(PROTO_OUT_DIR) --go-grpc_opt=paths=source_relative \
		-I$(PROTO_DIR) $(PROTO_DIR)/serverprobe.proto
	@echo "Proto files generated successfully"

# 编译 Agent
build: proto
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p ./bin
	go build -ldflags="-s -w" -o ./bin/$(BINARY_NAME) ./cmd/main.go
	@echo "Build complete: ./bin/$(BINARY_NAME)"

# 编译（多平台）
build-all: proto
	@echo "Building for multiple platforms..."
	@mkdir -p ./bin
	# Linux amd64
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ./bin/$(BINARY_NAME)-linux-amd64 ./cmd/main.go
	# Linux arm64
	GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o ./bin/$(BINARY_NAME)-linux-arm64 ./cmd/main.go
	# Windows amd64
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o ./bin/$(BINARY_NAME)-windows-amd64.exe ./cmd/main.go
	# macOS amd64
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o ./bin/$(BINARY_NAME)-darwin-amd64 ./cmd/main.go
	# macOS arm64
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o ./bin/$(BINARY_NAME)-darwin-arm64 ./cmd/main.go
	@echo "Multi-platform build complete"

# 清理
clean:
	@echo "Cleaning..."
	@rm -rf ./bin
	@rm -rf $(PROTO_OUT_DIR)/*.pb.go
	@echo "Clean complete"

# 运行测试
test:
	@echo "Running tests..."
	go test -v -race ./...

# 代码格式化
fmt:
	@echo "Formatting code..."
	go fmt ./...
	goimports -w .

# 代码检查
lint:
	@echo "Running linter..."
	golangci-lint run

# 运行 Agent
run: build
	@echo "Running $(BINARY_NAME)..."
	./bin/$(BINARY_NAME) --config=config.example.yaml

# 开发模式（自动重载）
dev:
	@which air > /dev/null || (echo "Installing air..." && go install github.com/cosmtrek/air@latest)
	@echo "Starting development mode with auto-reload..."
	air

# 安装到系统
install: build
	@echo "Installing $(BINARY_NAME)..."
	@sudo mkdir -p /etc/serverprobe
	@sudo cp ./bin/$(BINARY_NAME) /usr/local/bin/
	@sudo cp config.example.yaml /etc/serverprobe/agent.yaml.example
	@echo "Installation complete"
	@echo "Please configure /etc/serverprobe/agent.yaml before starting"

# 安装依赖
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy
	@echo "Dependencies installed"

# Docker 构建
docker-build:
	@echo "Building Docker image..."
	docker build -t serverprobe/agent:latest .
	@echo "Docker image built"

# 显示帮助
help:
	@echo "ServerProbe Agent Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make proto       - Generate protobuf code"
	@echo "  make build       - Build agent binary"
	@echo "  make build-all   - Build for all platforms"
	@echo "  make clean       - Clean build artifacts"
	@echo "  make test        - Run tests"
	@echo "  make fmt         - Format code"
	@echo "  make lint        - Run linter"
	@echo "  make run         - Build and run agent"
	@echo "  make dev         - Run in development mode with auto-reload"
	@echo "  make install     - Install to system"
	@echo "  make deps        - Install dependencies"
	@echo "  make docker-build - Build Docker image"
