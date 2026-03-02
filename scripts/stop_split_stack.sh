#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
NGINX_BIN="${NGINX_BIN:-/usr/local/nginx/sbin/nginx}"
NGINX_CONF="$SCRIPT_DIR/deploy/nginx/nginx.split.conf"
NGINX_PREFIX="$SCRIPT_DIR/.runtime/nginx"

echo "[1/2] 停止 nginx 网关..."
if [ -x "$NGINX_BIN" ]; then
  "$NGINX_BIN" -p "$NGINX_PREFIX/" -c "$NGINX_CONF" -s quit >/dev/null 2>&1 || true
fi

echo "[2/2] 停止后端服务..."
systemctl --user stop music-server >/dev/null 2>&1 || true
systemctl --user reset-failed music-server >/dev/null 2>&1 || true
systemctl --user stop auth-service >/dev/null 2>&1 || true
systemctl --user reset-failed auth-service >/dev/null 2>&1 || true
systemctl --user stop catalog-service >/dev/null 2>&1 || true
systemctl --user reset-failed catalog-service >/dev/null 2>&1 || true
systemctl --user stop profile-service >/dev/null 2>&1 || true
systemctl --user reset-failed profile-service >/dev/null 2>&1 || true
systemctl --user stop media-service >/dev/null 2>&1 || true
systemctl --user reset-failed media-service >/dev/null 2>&1 || true
systemctl --user stop video-service >/dev/null 2>&1 || true
systemctl --user reset-failed video-service >/dev/null 2>&1 || true
systemctl --user stop event-worker >/dev/null 2>&1 || true
systemctl --user reset-failed event-worker >/dev/null 2>&1 || true

echo "已停止 split 架构服务。"
