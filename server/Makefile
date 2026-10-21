.PHONY: all build test vet proto docker up down clean run install

BINARY_NAME=serverprobe
BIN_DIR=bin
CONFIG_FILE=config.yaml

all: build

build:
	@mkdir -p $(BIN_DIR)
	go build -ldflags="-s -w" -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/server
	@echo "Build complete: $(BIN_DIR)/$(BINARY_NAME)"

test:
	go test -count=1 ./...

vet:
	go vet ./...

# 需要安装 buf 与 protoc-gen-go/protoc-gen-go-grpc
proto:
	buf generate --template buf.gen.yaml ../proto

docker:
	docker build -t serverprobe:latest .

up:
	docker compose up -d

down:
	docker compose down

run:
	go run ./cmd/server -config $(CONFIG_FILE)

clean:
	rm -rf $(BIN_DIR)
	go clean -testcache

install: build
	@mkdir -p /opt/serverprobe
	install -m 0755 $(BIN_DIR)/$(BINARY_NAME) /opt/serverprobe/$(BINARY_NAME)
	install -m 0644 $(CONFIG_FILE) /opt/serverprobe/$(CONFIG_FILE)
	install -m 0644 scripts/serverprobe.service /etc/systemd/system/serverprobe.service
	systemctl daemon-reload
	systemctl enable serverprobe
	@echo "Installed. Configure /opt/serverprobe/config.yaml then run: systemctl start serverprobe"
