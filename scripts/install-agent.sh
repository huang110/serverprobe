#!/usr/bin/env bash
set -euo pipefail

SERVER_ADDR=""
AGENT_KEY=""
AGENT_NAME="$(hostname)"
TLS_MODE="none"
DOWNLOAD_URL="${AGENT_DOWNLOAD_URL:-https://github.com/huang110/serverprobe/releases/latest/download/agent-linux-amd64}"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --server) SERVER_ADDR="$2"; shift 2 ;;
        --key) AGENT_KEY="$2"; shift 2 ;;
        --name) AGENT_NAME="$2"; shift 2 ;;
        --tls-mode) TLS_MODE="$2"; shift 2 ;;
        --download-url) DOWNLOAD_URL="$2"; shift 2 ;;
        *) echo "未知参数: $1"; exit 1 ;;
    esac
done

if [ -z "$SERVER_ADDR" ] || [ -z "$AGENT_KEY" ]; then
    echo "用法: $0 --server <地址:端口> --key <注册密钥> [--name <名称>] [--tls-mode none|simple|mutual]"
    exit 1
fi

if [ "${EUID:-$(id -u)}" -ne 0 ]; then
    echo "请使用 root 权限运行"
    exit 1
fi

echo "=== 安装 ServerProbe Agent ==="

mkdir -p /opt/serverprobe/agent/scripts

if [ -f ./serverprobe-agent ]; then
    install -m 0755 ./serverprobe-agent /opt/serverprobe/agent/agent
elif [ -f ./agent ]; then
    install -m 0755 ./agent /opt/serverprobe/agent/agent
else
    echo "下载 Agent..."
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$DOWNLOAD_URL" -o /tmp/serverprobe-agent
    else
        wget -O /tmp/serverprobe-agent "$DOWNLOAD_URL"
    fi
    install -m 0755 /tmp/serverprobe-agent /opt/serverprobe/agent/agent
    rm -f /tmp/serverprobe-agent
fi

cat > /opt/serverprobe/agent/config.yaml <<EOF
agent:
  id: ""
  token: ""
  token_expires_at: 0
  lite_mode: false

server:
  address: "$SERVER_ADDR"
  register_secret: "$AGENT_KEY"
  tls_mode: "$TLS_MODE"
  ca_cert: ""
  client_cert: ""
  client_key: ""

monitor:
  heartbeat_interval: 5
  collect_interval: 5
  network_interfaces:
    - "eth*"
    - "ens*"
    - "en*"
  disk_mounts:
    - "/"
  exclude_fs_types:
    - "tmpfs"
    - "devtmpfs"
    - "squashfs"
    - "overlay"

log:
  level: "info"
  file: "/var/log/serverprobe/agent.log"
  max_size: 100
  max_days: 7

labels:
  hostname: "$AGENT_NAME"
EOF

mkdir -p /var/log/serverprobe
chmod 0600 /opt/serverprobe/agent/config.yaml

cat > /etc/systemd/system/serverprobe-agent.service <<'EOF'
[Unit]
Description=ServerProbe Agent
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/serverprobe/agent
ExecStart=/opt/serverprobe/agent/agent -config /opt/serverprobe/agent/config.yaml
Restart=always
RestartSec=10s
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable serverprobe-agent >/dev/null
systemctl restart serverprobe-agent
sleep 1
systemctl status serverprobe-agent --no-pager || true

echo "✅ Agent 安装完成"
