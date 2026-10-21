#!/usr/bin/env bash
set -euo pipefail

BACKUP_DIR="${BACKUP_DIR:-/var/backups/serverprobe}"
DATE="$(date +%Y%m%d_%H%M%S)"

DB_USER="${DB_USER:-serverprobe}"
DB_PASS="${DB_PASS:-serverprobe123}"
DB_NAME="${DB_NAME:-serverprobe}"
REDIS_ADDR="${REDIS_ADDR:-127.0.0.1:6379}"
REDIS_PASSWORD="${REDIS_PASSWORD:-}"

mkdir -p "$BACKUP_DIR"

echo "=== ServerProbe 备份 ==="

echo "备份 MySQL..."
mysqldump -u "$DB_USER" -p"$DB_PASS" "$DB_NAME" \
    --single-transaction \
    --routines \
    --triggers \
    | gzip > "$BACKUP_DIR/mysql_$DATE.sql.gz"

echo "备份 Redis..."
if [ -n "$REDIS_PASSWORD" ]; then
    redis-cli -h "${REDIS_ADDR%%:*}" -p "${REDIS_ADDR##*:}" -a "$REDIS_PASSWORD" --rdb "$BACKUP_DIR/redis_$DATE.rdb" >/dev/null
else
    redis-cli -h "${REDIS_ADDR%%:*}" -p "${REDIS_ADDR##*:}" --rdb "$BACKUP_DIR/redis_$DATE.rdb" >/dev/null
fi

echo "备份配置文件..."
CONFIG_FILES=()
[ -f /opt/serverprobe/server/config.yaml ] && CONFIG_FILES+=(/opt/serverprobe/server/config.yaml)
[ -f /opt/serverprobe/agent/config.yaml ] && CONFIG_FILES+=(/opt/serverprobe/agent/config.yaml)
if [ "${#CONFIG_FILES[@]}" -gt 0 ]; then
    tar czf "$BACKUP_DIR/config_$DATE.tar.gz" "${CONFIG_FILES[@]}"
fi

find "$BACKUP_DIR" -name "*.gz" -mtime +30 -delete
find "$BACKUP_DIR" -name "*.rdb" -mtime +30 -delete

echo "✅ 备份完成：$BACKUP_DIR"
ls -lh "$BACKUP_DIR"
