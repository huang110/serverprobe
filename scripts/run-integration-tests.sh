#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

echo "=== ServerProbe 集成测试 ==="

echo "[1/4] Server 单元测试与集成组件测试"
(cd server && go test -count=1 ./...)

echo "[2/4] 端到端集成测试（自动启动 Server + SQLite + miniredis）"
(cd tests && go test -v -count=1 -timeout 10m ./...)

echo "[3/4] Agent 单元测试"
(cd agent && go test -count=1 ./...)

echo "[4/4] 前端构建检查"
if [ -d frontend/node_modules ]; then
    (cd frontend && npm run build)
else
    echo "跳过前端构建：frontend/node_modules 不存在，请先执行 npm ci"
fi

if [ "${RUN_E2E:-0}" = "1" ]; then
    echo "运行 Playwright 前端测试..."
    (cd frontend && npm run test:e2e)
fi

echo ""
echo "✅ 集成测试完成"
