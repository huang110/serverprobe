#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if ! command -v docker >/dev/null 2>&1; then
    echo "错误：未安装 Docker"
    exit 1
fi

if ! docker compose version >/dev/null 2>&1 && ! command -v docker-compose >/dev/null 2>&1; then
    echo "错误：未安装 Docker Compose"
    exit 1
fi

COMPOSE="docker compose"
if ! docker compose version >/dev/null 2>&1; then
    COMPOSE="docker-compose"
fi

echo "=== ServerProbe 一键启动 ==="

if [ ! -f .env ]; then
    cp .env.example .env
    echo "已生成 .env，请按需修改密钥"
fi
if [ ! -f server/config.yaml ]; then
    cp server/config.example.yaml server/config.yaml
    echo "已生成 server/config.yaml"
fi
if [ ! -f agent/config.yaml ]; then
    cp agent/config.docker.yaml agent/config.yaml
    echo "已生成 agent/config.yaml"
fi

echo "构建镜像..."
$COMPOSE build

echo "启动服务..."
$COMPOSE up -d

echo "等待服务就绪..."
for i in $(seq 1 60); do
    if curl -fsS http://127.0.0.1:8080/api/health/ready >/dev/null 2>&1; then
        break
    fi
    sleep 2
done

$COMPOSE ps

echo ""
echo "✅ ServerProbe 已启动"
echo "  前端：http://localhost"
echo "  API： http://localhost:8080"
echo "  gRPC：localhost:9090"
echo "  默认登录：admin / admin123"
echo ""
echo "查看日志：$COMPOSE logs -f server"
echo "停止服务：./scripts/stop.sh"
