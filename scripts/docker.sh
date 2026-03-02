#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/docker-compose.yml"
ENV_FILE="$ROOT_DIR/.env.docker"
ENV_EXAMPLE="$ROOT_DIR/.env.docker.example"
RENDER_SCRIPT="$ROOT_DIR/scripts/docker/render_config.sh"

usage() {
  cat <<USAGE
Usage: ./scripts/docker.sh <command> [args]

Commands:
  up            Build and start full stack in background
  down          Stop and remove containers
  restart       Restart full stack
  logs [svc]    Follow logs (all services or specific service)
  ps            Show container status
  config        Render and print compose config
  migrate       Run migrator once
  build         Build app image only
  help          Show this help
USAGE
}

ensure_env_file() {
  if [[ ! -f "$ENV_FILE" ]]; then
    cp "$ENV_EXAMPLE" "$ENV_FILE"
    echo "Created $ENV_FILE from template."
    echo "Please review paths/passwords in $ENV_FILE before production use."
  fi
}

load_env_file() {
  set -a
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  set +a
}

ensure_data_dirs() {
  load_env_file
  (
    cd "$ROOT_DIR"
    mkdir -p \
      "${HOST_UPLOAD_DIR:-./.data/uploads}" \
      "${HOST_VIDEO_DIR:-./.data/video}" \
      "${HOST_HLS_DIR:-./.data/uploads_hls}"
  )
}

compose() {
  if docker compose version >/dev/null 2>&1; then
    docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" "$@"
    return
  fi

  if command -v docker-compose >/dev/null 2>&1; then
    docker-compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" "$@"
    return
  fi

  echo "Error: docker compose is not available. Install Docker Engine + Compose plugin first."
  exit 1
}

render_config() {
  "$RENDER_SCRIPT" "$ENV_FILE"
}

cmd="${1:-up}"
shift || true

case "$cmd" in
  up)
    ensure_env_file
    render_config
    ensure_data_dirs
    compose build "$@"
    compose up -d "$@"
    echo "Gateway: http://127.0.0.1:${GATEWAY_PORT:-8080}"
    ;;
  down)
    ensure_env_file
    compose down "$@"
    ;;
  restart)
    ensure_env_file
    render_config
    ensure_data_dirs
    compose down
    compose build "$@"
    compose up -d "$@"
    ;;
  logs)
    ensure_env_file
    compose logs -f --tail=200 "$@"
    ;;
  ps)
    ensure_env_file
    compose ps "$@"
    ;;
  config)
    ensure_env_file
    render_config
    compose config "$@"
    ;;
  migrate)
    ensure_env_file
    render_config
    compose run --rm migrator "$@"
    ;;
  build)
    ensure_env_file
    render_config
    compose build "$@"
    ;;
  help|-h|--help)
    usage
    ;;
  *)
    usage
    exit 1
    ;;
esac
