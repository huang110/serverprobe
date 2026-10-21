#!/usr/bin/env bash
set -euo pipefail

if [ "${EUID:-$(id -u)}" -ne 0 ]; then
    echo "请使用 root 权限运行"
    exit 1
fi

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INSTALL_DIR="/opt/serverprobe"

echo "=== ServerProbe 生产环境安装 ==="

if ! id -u serverprobe >/dev/null 2>&1; then
    useradd -r -s /usr/sbin/nologin serverprobe || useradd -r -s /bin/false serverprobe
fi

mkdir -p "$INSTALL_DIR/server/logs" "$INSTALL_DIR/agent/scripts" "$INSTALL_DIR/scripts"

echo "编译 Server..."
(cd "$ROOT/server" && CGO_ENABLED=0 go build -ldflags="-s -w" -o "$INSTALL_DIR/server/server" ./cmd/server)

echo "编译 Agent..."
(cd "$ROOT/agent" && CGO_ENABLED=0 go build -ldflags="-s -w" -o "$INSTALL_DIR/agent/agent" ./cmd)

echo "安装配置..."
if [ ! -f "$INSTALL_DIR/server/config.yaml" ]; then
    cp "$ROOT/server/config.production.yaml" "$INSTALL_DIR/server/config.yaml"
fi
if [ ! -f "$INSTALL_DIR/agent/config.yaml" ]; then
    cp "$ROOT/agent/config.example.yaml" "$INSTALL_DIR/agent/config.yaml"
fi
install -m 0755 "$ROOT/scripts/backup.sh" "$INSTALL_DIR/scripts/backup.sh"
install -m 0644 "$ROOT/scripts/logrotate.serverprobe" /etc/logrotate.d/serverprobe

chown -R serverprobe:serverprobe "$INSTALL_DIR/server"
chown -R root:root "$INSTALL_DIR/agent"
chmod 0755 "$INSTALL_DIR/server/server" "$INSTALL_DIR/agent/agent"
chmod 0600 "$INSTALL_DIR/server/config.yaml" "$INSTALL_DIR/agent/config.yaml"

install -m 0644 "$ROOT/scripts/serverprobe-server.service" /etc/systemd/system/serverprobe-server.service
install -m 0644 "$ROOT/scripts/serverprobe-agent.service" /etc/systemd/system/serverprobe-agent.service
systemctl daemon-reload

systemctl enable serverprobe-server >/dev/null
systemctl enable serverprobe-agent >/dev/null
systemctl restart serverprobe-server || true
systemctl restart serverprobe-agent || true

sleep 2
systemctl status serverprobe-server --no-pager || true
systemctl status serverprobe-agent --no-pager || true

echo ""
echo "✅ 安装完成"
echo "请修改 $INSTALL_DIR/server/config.yaml 中的数据库、Redis、JWT 和管理员密码"
echo "然后执行：systemctl restart serverprobe-server"
