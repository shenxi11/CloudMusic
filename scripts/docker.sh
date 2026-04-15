#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/docker-compose.yml"
ENV_FILE="$ROOT_DIR/.env.docker"
ENV_EXAMPLE="$ROOT_DIR/.env.docker.example"
RENDER_SCRIPT="$ROOT_DIR/scripts/docker/render_config.sh"

export DOCKER_BUILDKIT="${DOCKER_BUILDKIT:-1}"

usage() {
  cat <<USAGE
Usage: ./scripts/docker.sh <command> [args]

Commands:
  up [--no-build]
                Build and start full stack in background
  down          Stop and remove containers
  restart [--no-build]
                Restart full stack
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

build_enabled() {
  if [[ "${SKIP_DOCKER_BUILD:-}" == "1" ]]; then
    return 1
  fi
  return 0
}

strip_no_build_flag() {
  local -n args_ref=$1
  local out=()
  for arg in "${args_ref[@]}"; do
    case "$arg" in
      --no-build)
        SKIP_DOCKER_BUILD=1
        ;;
      *)
        out+=("$arg")
        ;;
    esac
  done
  args_ref=("${out[@]}")
}

cmd="${1:-up}"
shift || true
args=("$@")
strip_no_build_flag args

case "$cmd" in
  up)
    ensure_env_file
    enforce_prod_repo_guard
    render_config
    ensure_data_dirs
    if build_enabled; then
      compose build "${args[@]}"
    else
      echo "Skipping image build because --no-build or SKIP_DOCKER_BUILD=1 was set."
    fi
    compose up -d "${args[@]}"
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
    if build_enabled; then
      compose build "${args[@]}"
    else
      echo "Skipping image build because --no-build or SKIP_DOCKER_BUILD=1 was set."
    fi
    compose up -d "${args[@]}"
    ;;
  logs)
    ensure_env_file
    compose logs -f --tail=200 "${args[@]}"
    ;;
  ps)
    ensure_env_file
    compose ps "${args[@]}"
    ;;
  config)
    ensure_env_file
    render_config
    compose config "${args[@]}"
    ;;
  migrate)
    ensure_env_file
    enforce_prod_repo_guard
    render_config
    compose run --rm migrator "${args[@]}"
    ;;
  sync-media)
    ensure_env_file
    render_config
    ensure_data_dirs
    if [[ ${#args[@]} -eq 0 ]]; then
      compose run --rm music-server /app/media_indexer \
        -config /app/configs/config.yaml \
        -audio-dir /data/uploads \
        -video-dir /data/video
    else
      compose run --rm music-server /app/media_indexer "${args[@]}"
    fi
    ;;
  build)
    ensure_env_file
    render_config
    compose build "${args[@]}"
    ;;
  help|-h|--help)
    usage
    ;;
  *)
    usage
    exit 1
    ;;
esac
