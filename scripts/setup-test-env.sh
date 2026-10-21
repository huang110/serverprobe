#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

echo "=== ServerProbe 测试环境检查 ==="

need() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "缺少依赖：$1"
        return 1
    fi
    echo "OK：$1"
}

STATUS=0
need go || STATUS=1
need node || STATUS=1
need npm || STATUS=1

# Docker / MySQL / Redis 对自包含集成测试不是必需，仅提示
command -v docker >/dev/null 2>&1 && echo "OK：docker" || echo "提示：未安装 Docker，将跳过后端容器测试"
command -v mysql >/dev/null 2>&1 && echo "OK：mysql client" || echo "提示：未安装 MySQL client"
command -v redis-cli >/dev/null 2>&1 && echo "OK：redis-cli" || echo "提示：未安装 redis-cli"

if [ "$STATUS" -ne 0 ]; then
    echo "请先安装缺失的基础依赖"
    exit 1
fi

if [ ! -f server/config.yaml ]; then
    cp server/config.example.yaml server/config.yaml
    echo "已生成 server/config.yaml"
fi

if [ ! -f agent/config.yaml ]; then
    cp agent/config.example.yaml agent/config.yaml
    echo "已生成 agent/config.yaml"
fi

if [ ! -f .env ]; then
    cp .env.example .env
    echo "已生成 .env"
fi

echo "检查 Go 依赖..."
(cd server && go mod download)
(cd agent && go mod download)
(cd tests && go mod download)

echo ""
echo "测试环境准备完成。"
echo "运行集成测试：./scripts/run-integration-tests.sh"
echo "启动 Docker 环境：./scripts/start.sh"
