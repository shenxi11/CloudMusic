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
  sync-media    Scan media dirs and upsert DB metadata (passes args to media_indexer)
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

enforce_prod_repo_guard() {
  load_env_file
  local project_name
  project_name="${COMPOSE_PROJECT_NAME:-$(basename "$ROOT_DIR")}"
  if [[ "$project_name" != "cloudmusic" ]]; then
    return
  fi

  if ! command -v git >/dev/null 2>&1; then
    echo "Error: git is required for the cloudmusic production workflow."
    exit 1
  fi

  local branch
  branch="$(git -C "$ROOT_DIR" branch --show-current)"
  if [[ "$branch" != "main" ]]; then
    echo "Error: cloudmusic 正式运行目录只能在 main 分支部署。当前分支: $branch"
    echo "请先执行: git fetch origin && git checkout main && git pull --ff-only origin main"
    exit 1
  fi

  local dirty
  dirty="$(git -C "$ROOT_DIR" status --porcelain)"
  if [[ -n "$dirty" ]]; then
    echo "Error: cloudmusic 正式运行目录工作区必须干净后才能部署。"
    echo "$dirty"
    exit 1
  fi

  if ! git -C "$ROOT_DIR" ls-remote --exit-code origin HEAD >/dev/null 2>&1; then
    echo "Error: 无法访问 origin，请先检查网络或远端配置。"
    exit 1
  fi
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

resolve_project_name() {
  load_env_file
  local fallback
  fallback="$(basename "$ROOT_DIR")"
  if [[ -n "${COMPOSE_PROJECT_NAME:-}" ]]; then
    printf '%s' "$COMPOSE_PROJECT_NAME"
    return
  fi
  printf '%s' "$fallback"
}

compose() {
  local project_name
  project_name="$(resolve_project_name)"

  if docker compose version >/dev/null 2>&1; then
    docker compose -p "$project_name" --env-file "$ENV_FILE" -f "$COMPOSE_FILE" "$@"
    return
  fi

  if command -v docker-compose >/dev/null 2>&1; then
    docker-compose -p "$project_name" --env-file "$ENV_FILE" -f "$COMPOSE_FILE" "$@"
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
    enforce_prod_repo_guard
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
    enforce_prod_repo_guard
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
    enforce_prod_repo_guard
    render_config
    compose run --rm migrator "$@"
    ;;
  sync-media)
    ensure_env_file
    render_config
    ensure_data_dirs
    if [[ $# -eq 0 ]]; then
      compose run --rm music-server /app/media_indexer \
        -config /app/configs/config.yaml \
        -audio-dir /data/uploads \
        -video-dir /data/video
    else
      compose run --rm music-server /app/media_indexer "$@"
    fi
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
