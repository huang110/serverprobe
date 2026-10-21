# ServerProbe Server

ServerProbe 后端服务，负责 Agent 注册、指标接收、存储、查询、实时推送和告警评估。

## 技术栈

- Go 1.21+
- Gin
- gRPC + Protocol Buffers
- GORM（MySQL / SQLite）
- Redis
- Gorilla WebSocket

## 目录结构

```text
server/
├── cmd/server/          # 主程序入口
├── internal/
│   ├── aggregator/      # 数据聚合与清理
│   ├── alert/           # 告警引擎
│   ├── api/handler/     # HTTP 接口
│   ├── cache/           # Redis 缓存与 Pub/Sub
│   ├── config/          # Viper 配置
│   ├── grpc/            # gRPC 服务
│   ├── middleware/      # JWT / CORS / 限流 / 日志
│   ├── model/           # GORM 模型与迁移
│   ├── pkg/auth/        # HMAC 签名
│   └── websocket/       # WebSocket Hub
├── proto/               # 生成的 proto 代码
├── config.example.yaml
├── config.production.yaml
├── Dockerfile
└── Makefile
```

## 本地运行

```bash
cp config.example.yaml config.yaml
go run ./cmd/server -config config.yaml
```

默认端口：HTTP `8080`，gRPC `9090`。

## 测试

```bash
go test -count=1 ./...
go vet ./...
```

完整 Docker 部署请参考仓库根目录的 `docker-compose.yml` 和 `docs/DEPLOYMENT.md`。
