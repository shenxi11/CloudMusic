#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DOCS_DIR="$ROOT_DIR/docs"
RUNTIME_DIR="$ROOT_DIR/.runtime"
PID_FILE="$RUNTIME_DIR/openapi_preview.pid"
LOG_FILE="$RUNTIME_DIR/openapi_preview.log"

HOST="${OPENAPI_PREVIEW_HOST:-127.0.0.1}"
PORT="${OPENAPI_PREVIEW_PORT:-18090}"
ACTION="${1:-start}"

usage() {
  cat <<USAGE
Usage:
  ./scripts/openapi_preview.sh [start|stop|status]

Environment variables:
  OPENAPI_PREVIEW_HOST   default: 127.0.0.1
  OPENAPI_PREVIEW_PORT   default: 18090
USAGE
}

require_files() {
  if ! command -v python3 >/dev/null 2>&1; then
    echo "error: python3 not found"
    exit 1
  fi
  if [ ! -f "$DOCS_DIR/openapi.yaml" ]; then
    echo "error: missing $DOCS_DIR/openapi.yaml"
    exit 1
  fi
  if [ ! -f "$DOCS_DIR/swagger-ui.html" ]; then
    echo "error: missing $DOCS_DIR/swagger-ui.html"
    exit 1
  fi
}

is_running() {
  if [ ! -f "$PID_FILE" ]; then
    return 1
  fi
  local pid
  pid="$(cat "$PID_FILE" 2>/dev/null || true)"
  if [ -z "$pid" ]; then
    return 1
  fi
  if kill -0 "$pid" 2>/dev/null; then
    return 0
  fi
  return 1
}

start_server() {
  require_files
  mkdir -p "$RUNTIME_DIR"

  if is_running; then
    echo "openapi preview is already running (pid=$(cat "$PID_FILE"))."
    echo "swagger ui: http://$HOST:$PORT/swagger-ui.html"
    return 0
  fi

  (
    cd "$DOCS_DIR"
    nohup python3 -m http.server "$PORT" --bind "$HOST" >"$LOG_FILE" 2>&1 &
    echo $! >"$PID_FILE"
  )

  sleep 0.2
  if ! is_running; then
    echo "error: failed to start preview server"
    [ -f "$LOG_FILE" ] && tail -n 50 "$LOG_FILE"
    exit 1
  fi

  echo "openapi preview started (pid=$(cat "$PID_FILE"))."
  echo "swagger ui: http://$HOST:$PORT/swagger-ui.html"
  echo "openapi yaml: http://$HOST:$PORT/openapi.yaml"
  echo "log: $LOG_FILE"
}

stop_server() {
  if ! is_running; then
    echo "openapi preview is not running."
    rm -f "$PID_FILE"
    return 0
  fi

  local pid
  pid="$(cat "$PID_FILE")"
  kill "$pid" 2>/dev/null || true
  sleep 0.2
  if kill -0 "$pid" 2>/dev/null; then
    kill -9 "$pid" 2>/dev/null || true
  fi
  rm -f "$PID_FILE"
  echo "openapi preview stopped."
}

status_server() {
  if is_running; then
    echo "openapi preview is running (pid=$(cat "$PID_FILE"))."
    echo "swagger ui: http://$HOST:$PORT/swagger-ui.html"
  else
    echo "openapi preview is not running."
  fi
}

case "$ACTION" in
  start)
    start_server
    ;;
  stop)
    stop_server
    ;;
  status)
    status_server
    ;;
  -h|--help|help)
    usage
    ;;
  *)
    usage
    exit 1
    ;;
esac
