#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
NGINX_BIN="${NGINX_BIN:-/usr/local/nginx/sbin/nginx}"
NGINX_TEMPLATE="$SCRIPT_DIR/deploy/nginx/nginx.split.conf"
NGINX_PREFIX="$SCRIPT_DIR/.runtime/nginx"
NGINX_CONF="$NGINX_PREFIX/nginx.split.conf"
MIGRATOR_SCRIPT="$SCRIPT_DIR/scripts/migrate_all.sh"
BACKEND_HEALTH_URL="http://127.0.0.1:18080/health"
AUTH_HEALTH_URL="http://127.0.0.1:18081/health"
CATALOG_HEALTH_URL="http://127.0.0.1:18082/health"
PROFILE_HEALTH_URL="http://127.0.0.1:18083/health"
MEDIA_HEALTH_URL="http://127.0.0.1:18084/health"
VIDEO_HEALTH_URL="http://127.0.0.1:18085/health"
GATEWAY_HEALTH_URL="http://127.0.0.1:8080/health"
UPLOADS_ROOT="${UPLOADS_ROOT:-$SCRIPT_DIR/uploads}"
VIDEO_ROOT="${VIDEO_ROOT:-$SCRIPT_DIR/video}"
HLS_ROOT="${HLS_ROOT:-$SCRIPT_DIR/uploads_hls}"
VIDEO_HLS_ROOT="${VIDEO_HLS_ROOT:-$SCRIPT_DIR/video_hls}"

render_nginx_conf() {
  python3 - "$NGINX_TEMPLATE" "$NGINX_CONF" "$UPLOADS_ROOT" "$VIDEO_ROOT" "$HLS_ROOT" "$VIDEO_HLS_ROOT" <<'PY'
from pathlib import Path
import sys

template_path = Path(sys.argv[1])
output_path = Path(sys.argv[2])
uploads_root = Path(sys.argv[3]).resolve().as_posix().rstrip('/') + '/'
video_root = Path(sys.argv[4]).resolve().as_posix().rstrip('/') + '/'
hls_root = Path(sys.argv[5]).resolve().as_posix().rstrip('/') + '/'
text = template_path.read_text(encoding='utf-8')
text = text.replace('__UPLOADS_ROOT__', uploads_root)
text = text.replace('__VIDEO_ROOT__', video_root)
text = text.replace('__HLS_ROOT__', hls_root)
text = text.replace('__VIDEO_HLS_ROOT__', video_hls_root)
output_path.write_text(text, encoding='utf-8')
PY
}

echo "[1/11] 检查依赖..."
if [ ! -x "$NGINX_BIN" ]; then
  echo "错误: 未找到 nginx 可执行文件: $NGINX_BIN"
  exit 1
fi
if [ ! -f "$SCRIPT_DIR/music_server" ]; then
  echo "错误: 未找到后端二进制: $SCRIPT_DIR/music_server"
  exit 1
fi
if [ ! -f "$SCRIPT_DIR/auth_server" ]; then
  echo "错误: 未找到认证服务二进制: $SCRIPT_DIR/auth_server"
  echo "请先执行: go build -ldflags='-s -w' -o auth_server cmd/auth/main.go"
  exit 1
fi
if [ ! -f "$SCRIPT_DIR/catalog_server" ]; then
  echo "错误: 未找到内容服务二进制: $SCRIPT_DIR/catalog_server"
  echo "请先执行: go build -ldflags='-s -w' -o catalog_server cmd/catalog/main.go"
  exit 1
fi
if [ ! -f "$SCRIPT_DIR/profile_server" ]; then
  echo "错误: 未找到用户行为服务二进制: $SCRIPT_DIR/profile_server"
  echo "请先执行: go build -ldflags='-s -w' -o profile_server cmd/profile/main.go"
  exit 1
fi
if [ ! -f "$SCRIPT_DIR/media_server" ]; then
  echo "错误: 未找到媒体服务二进制: $SCRIPT_DIR/media_server"
  echo "请先执行: go build -ldflags='-s -w' -o media_server cmd/media/main.go"
  exit 1
fi
if [ ! -f "$SCRIPT_DIR/video_server" ]; then
  echo "错误: 未找到视频服务二进制: $SCRIPT_DIR/video_server"
  echo "请先执行: go build -ldflags='-s -w' -o video_server cmd/video/main.go"
  exit 1
fi
if [ ! -f "$SCRIPT_DIR/event_worker" ]; then
  echo "错误: 未找到事件消费器二进制: $SCRIPT_DIR/event_worker"
  echo "请先执行: go build -ldflags='-s -w' -o event_worker cmd/eventworker/main.go"
  exit 1
fi
if [ ! -x "$MIGRATOR_SCRIPT" ]; then
  echo "错误: 未找到迁移脚本: $MIGRATOR_SCRIPT"
  echo "请先执行: chmod +x scripts/migrate_all.sh"
  exit 1
fi
if [ ! -f "$NGINX_TEMPLATE" ]; then
  echo "错误: 未找到 nginx 配置模板: $NGINX_TEMPLATE"
  exit 1
fi

echo "[2/11] 准备 nginx 运行目录..."
mkdir -p \
  "$NGINX_PREFIX/logs" \
  "$NGINX_PREFIX/temp/client_body" \
  "$NGINX_PREFIX/temp/proxy" \
  "$NGINX_PREFIX/temp/fastcgi" \
  "$NGINX_PREFIX/temp/uwsgi" \
  "$NGINX_PREFIX/temp/scgi" \
  "$HLS_ROOT"
  "$VIDEO_HLS_ROOT"
render_nginx_conf

if [ ! -d "$UPLOADS_ROOT" ]; then
  echo "错误: 未找到静态 uploads 目录: $UPLOADS_ROOT"
  exit 1
fi

echo "[3/11] 执行数据库迁移..."
"$MIGRATOR_SCRIPT"

echo "[4/11] 启动业务后端（127.0.0.1:18080）..."
systemctl --user stop music-server >/dev/null 2>&1 || true
systemctl --user reset-failed music-server >/dev/null 2>&1 || true
systemd-run --user --unit=music-server \
  --property=WorkingDirectory="$SCRIPT_DIR" \
  "$SCRIPT_DIR/music_server" >/dev/null

for i in $(seq 1 30); do
  if curl -fsS -m 1 "$BACKEND_HEALTH_URL" >/dev/null 2>&1; then
    break
  fi
  sleep 0.2
done
if ! curl -fsS -m 2 "$BACKEND_HEALTH_URL" >/dev/null 2>&1; then
  echo "错误: 后端健康检查失败: $BACKEND_HEALTH_URL"
  exit 1
fi

echo "[5/11] 启动认证服务（127.0.0.1:18081）..."
systemctl --user stop auth-service >/dev/null 2>&1 || true
systemctl --user reset-failed auth-service >/dev/null 2>&1 || true
systemd-run --user --unit=auth-service \
  --property=WorkingDirectory="$SCRIPT_DIR" \
  "$SCRIPT_DIR/auth_server" >/dev/null

for i in $(seq 1 30); do
  if curl -fsS -m 1 "$AUTH_HEALTH_URL" >/dev/null 2>&1; then
    break
  fi
  sleep 0.2
done
if ! curl -fsS -m 2 "$AUTH_HEALTH_URL" >/dev/null 2>&1; then
  echo "错误: 认证服务健康检查失败: $AUTH_HEALTH_URL"
  exit 1
fi

echo "[6/11] 启动内容服务（127.0.0.1:18082）..."
systemctl --user stop catalog-service >/dev/null 2>&1 || true
systemctl --user reset-failed catalog-service >/dev/null 2>&1 || true
systemd-run --user --unit=catalog-service \
  --property=WorkingDirectory="$SCRIPT_DIR" \
  "$SCRIPT_DIR/catalog_server" >/dev/null

for i in $(seq 1 30); do
  if curl -fsS -m 1 "$CATALOG_HEALTH_URL" >/dev/null 2>&1; then
    break
  fi
  sleep 0.2
done
if ! curl -fsS -m 2 "$CATALOG_HEALTH_URL" >/dev/null 2>&1; then
  echo "错误: 内容服务健康检查失败: $CATALOG_HEALTH_URL"
  exit 1
fi

echo "[7/11] 启动用户行为服务（127.0.0.1:18083）..."
systemctl --user stop profile-service >/dev/null 2>&1 || true
systemctl --user reset-failed profile-service >/dev/null 2>&1 || true
systemd-run --user --unit=profile-service \
  --property=WorkingDirectory="$SCRIPT_DIR" \
  "$SCRIPT_DIR/profile_server" >/dev/null

for i in $(seq 1 30); do
  if curl -fsS -m 1 "$PROFILE_HEALTH_URL" >/dev/null 2>&1; then
    break
  fi
  sleep 0.2
done
if ! curl -fsS -m 2 "$PROFILE_HEALTH_URL" >/dev/null 2>&1; then
  echo "错误: 用户行为服务健康检查失败: $PROFILE_HEALTH_URL"
  exit 1
fi

echo "[8/11] 启动媒体服务（127.0.0.1:18084）..."
systemctl --user stop media-service >/dev/null 2>&1 || true
systemctl --user reset-failed media-service >/dev/null 2>&1 || true
systemd-run --user --unit=media-service \
  --property=WorkingDirectory="$SCRIPT_DIR" \
  "$SCRIPT_DIR/media_server" >/dev/null

for i in $(seq 1 30); do
  if curl -fsS -m 1 "$MEDIA_HEALTH_URL" >/dev/null 2>&1; then
    break
  fi
  sleep 0.2
done
if ! curl -fsS -m 2 "$MEDIA_HEALTH_URL" >/dev/null 2>&1; then
  echo "错误: 媒体服务健康检查失败: $MEDIA_HEALTH_URL"
  exit 1
fi

echo "[9/11] 启动视频服务（127.0.0.1:18085）..."
systemctl --user stop video-service >/dev/null 2>&1 || true
systemctl --user reset-failed video-service >/dev/null 2>&1 || true
systemd-run --user --unit=video-service \
  --property=WorkingDirectory="$SCRIPT_DIR" \
  "$SCRIPT_DIR/video_server" >/dev/null

for i in $(seq 1 30); do
  if curl -fsS -m 1 "$VIDEO_HEALTH_URL" >/dev/null 2>&1; then
    break
  fi
  sleep 0.2
done
if ! curl -fsS -m 2 "$VIDEO_HEALTH_URL" >/dev/null 2>&1; then
  echo "错误: 视频服务健康检查失败: $VIDEO_HEALTH_URL"
  exit 1
fi

echo "[10/11] 启动事件消费器..."
systemctl --user stop event-worker >/dev/null 2>&1 || true
systemctl --user reset-failed event-worker >/dev/null 2>&1 || true
systemd-run --user --unit=event-worker \
  --property=WorkingDirectory="$SCRIPT_DIR" \
  "$SCRIPT_DIR/event_worker" >/dev/null

sleep 0.5
if ! systemctl --user is-active --quiet event-worker; then
  echo "错误: 事件消费器启动失败"
  systemctl --user status --no-pager event-worker || true
  exit 1
fi

echo "[11/11] 启动/重载 nginx 网关（*:8080）..."
if [ -f "$NGINX_PREFIX/logs/nginx.pid" ] && kill -0 "$(cat "$NGINX_PREFIX/logs/nginx.pid")" 2>/dev/null; then
  "$NGINX_BIN" -p "$NGINX_PREFIX/" -c "$NGINX_CONF" -s reload
else
  "$NGINX_BIN" -p "$NGINX_PREFIX/" -c "$NGINX_CONF"
fi

for i in $(seq 1 30); do
  if curl -fsS -m 1 "$GATEWAY_HEALTH_URL" >/dev/null 2>&1; then
    break
  fi
  sleep 0.2
done
if ! curl -fsS -m 2 "$GATEWAY_HEALTH_URL" >/dev/null 2>&1; then
  echo "错误: 网关健康检查失败: $GATEWAY_HEALTH_URL"
  exit 1
fi

echo "启动完成"
echo "  后端健康: $BACKEND_HEALTH_URL"
echo "  认证健康: $AUTH_HEALTH_URL"
echo "  内容健康: $CATALOG_HEALTH_URL"
echo "  用户行为健康: $PROFILE_HEALTH_URL"
echo "  媒体健康: $MEDIA_HEALTH_URL"
echo "  视频健康: $VIDEO_HEALTH_URL"
echo "  事件消费器: active"
echo "  对外健康: $GATEWAY_HEALTH_URL"
echo "  静态 uploads: $UPLOADS_ROOT"
echo "  静态 hls: $HLS_ROOT"
echo "  静态 video hls: $VIDEO_HLS_ROOT"
echo "  静态 video: $VIDEO_ROOT"
echo "  资源入口: http://127.0.0.1:8080/uploads/... 和 /video/..."
