#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="$ROOT_DIR/.env.docker"
START_SCRIPT="$ROOT_DIR/start_docker.sh"

DEPLOY_MODE="auto"
REFRESH_BASE=0
START_ARGS=()
CHANGED_FILES=()
DEPLOY_STRATEGY=""

BASE_BUILD_REQUIRED_PATTERNS=(
  "Dockerfile.runtime-base"
)

APP_BUILD_REQUIRED_PATTERNS=(
  ".dockerignore"
  "Dockerfile"
  "docker-compose.yml"
  "go.mod"
  "go.sum"
  "cmd/"
  "internal/"
  "pkg/"
  "migrations/sql/"
  "scripts/docker/wait_for.sh"
  "configs/config.docker.yaml"
)

RECREATE_ONLY_PATTERNS=(
  "deploy/nginx/nginx.docker.conf"
  "scripts/docker/render_config.sh"
)

fail() {
  echo "Error: $*" >&2
  exit 1
}

usage() {
  cat <<USAGE
Usage: ./scripts/deploy_from_main.sh [--force-build | --no-build] [--refresh-base] [extra start_docker args]

Modes:
  default / auto  自动根据拉取到的文件变化决定:
                  - 仅文档/辅助文件变化: 不重启
                  - 仅运行时配置变化:   --no-build --force-recreate
                  - 仅应用镜像变化:     --force-recreate
                  - 基础运行时镜像变化: --refresh-base --force-recreate
  --force-build   无条件执行应用镜像构建后部署
  --no-build      无条件跳过构建并重建容器
  --refresh-base  强制刷新 ffmpeg 运行时基础镜像
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --force-build)
      [[ "$DEPLOY_MODE" == "auto" ]] || fail "不能同时指定 --force-build 和 --no-build"
      DEPLOY_MODE="force-build"
      ;;
    --no-build)
      [[ "$DEPLOY_MODE" == "auto" ]] || fail "不能同时指定 --force-build 和 --no-build"
      DEPLOY_MODE="no-build"
      ;;
    --refresh-base)
      REFRESH_BASE=1
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      START_ARGS+=("$1")
      ;;
  esac
  shift
done

matches_prefix() {
  local path="$1"
  shift
  local pattern
  for pattern in "$@"; do
    if [[ "$pattern" == */ ]]; then
      [[ "$path" == "$pattern"* ]] && return 0
      continue
    fi
    [[ "$path" == "$pattern" ]] && return 0
  done
  return 1
}

detect_deploy_strategy() {
  local old_head="$1"
  local new_head="$2"

  CHANGED_FILES=()
  mapfile -t CHANGED_FILES < <(git -C "$ROOT_DIR" diff --name-only "$old_head..$new_head")

  if [[ ${#CHANGED_FILES[@]} -eq 0 ]]; then
    DEPLOY_STRATEGY="no-change"
    return 0
  fi

  local file
  local requires_base_build=0
  local requires_app_build=0
  local requires_recreate=0
  local has_runtime_change=0

  for file in "${CHANGED_FILES[@]}"; do
    if matches_prefix "$file" "${BASE_BUILD_REQUIRED_PATTERNS[@]}"; then
      requires_base_build=1
      requires_app_build=1
      has_runtime_change=1
      continue
    fi
    if matches_prefix "$file" "${APP_BUILD_REQUIRED_PATTERNS[@]}"; then
      requires_app_build=1
      has_runtime_change=1
      continue
    fi
    if matches_prefix "$file" "${RECREATE_ONLY_PATTERNS[@]}"; then
      requires_recreate=1
      has_runtime_change=1
    fi
  done

  if (( requires_base_build )); then
    DEPLOY_STRATEGY="base-build-required"
    return 0
  fi
  if (( requires_app_build )); then
    DEPLOY_STRATEGY="app-build-required"
    return 0
  fi
  if (( requires_recreate )); then
    DEPLOY_STRATEGY="recreate-only"
    return 0
  fi
  if (( has_runtime_change == 0 )); then
    DEPLOY_STRATEGY="no-runtime-impact"
    return 0
  fi

  DEPLOY_STRATEGY="no-runtime-impact"
}

print_changed_files() {
  if [[ ${#CHANGED_FILES[@]} -eq 0 ]]; then
    echo "Changed files: <none>"
    return 0
  fi
  echo "Changed files:"
  printf '  %s\n' "${CHANGED_FILES[@]}"
}

run_start_script() {
  local mode="$1"
  shift || true

  local extra_args=("$@")
  if [[ "$mode" == "base-build-required" || "$REFRESH_BASE" == "1" ]]; then
    extra_args=(--refresh-base "${extra_args[@]}")
  fi

  case "$mode" in
    base-build-required)
      echo "[4/4] deploy cloudmusic via start_docker.sh --refresh-base --force-recreate"
      "$START_SCRIPT" "${extra_args[@]}" --force-recreate
      ;;
    app-build-required|force-build)
      if [[ "$REFRESH_BASE" == "1" ]]; then
        echo "[4/4] deploy cloudmusic via start_docker.sh --refresh-base --force-recreate"
      else
        echo "[4/4] deploy cloudmusic via start_docker.sh --force-recreate"
      fi
      "$START_SCRIPT" "${extra_args[@]}" --force-recreate
      ;;
    recreate-only|no-build)
      echo "[4/4] deploy cloudmusic via start_docker.sh --no-build --force-recreate"
      "$START_SCRIPT" --no-build --force-recreate "$@"
      ;;
    no-runtime-impact)
      echo "[4/4] no runtime-impact changes detected; skip container restart"
      return 0
      ;;
    no-change)
      echo "[4/4] no new commits detected after pull; skip deployment"
      return 0
      ;;
    *)
      fail "未知部署策略: $mode"
      ;;
  esac
}

if [[ "$DEPLOY_MODE" == "no-build" && "$REFRESH_BASE" == "1" ]]; then
  fail "--refresh-base 不能和 --no-build 同时使用"
fi

command -v git >/dev/null 2>&1 || fail "git 未安装"
[[ -f "$ENV_FILE" ]] || fail "$ENV_FILE 不存在"
[[ -x "$START_SCRIPT" ]] || fail "$START_SCRIPT 不可执行"

branch="$(git -C "$ROOT_DIR" branch --show-current)"
if [[ "$branch" != "main" ]]; then
  fail "CloudMusic 正式运行目录必须在 main 分支。当前分支: $branch"
fi

if [[ -n "$(git -C "$ROOT_DIR" status --porcelain)" ]]; then
  git -C "$ROOT_DIR" status --short
  fail "CloudMusic 工作区不干净，请先清理后再部署"
fi

if ! git -C "$ROOT_DIR" ls-remote --exit-code origin HEAD >/dev/null 2>&1; then
  fail "无法访问 origin，请先检查仓库远端和网络"
fi

old_head="$(git -C "$ROOT_DIR" rev-parse HEAD)"

echo "[1/4] fetch origin"
git -C "$ROOT_DIR" fetch origin

echo "[2/4] checkout main"
git -C "$ROOT_DIR" checkout main

echo "[3/4] pull latest main"
git -C "$ROOT_DIR" pull --ff-only origin main

new_head="$(git -C "$ROOT_DIR" rev-parse HEAD)"

if [[ "$new_head" == "$old_head" && "$DEPLOY_MODE" == "auto" ]]; then
  DEPLOY_STRATEGY="no-change"
else
  case "$DEPLOY_MODE" in
    auto)
      echo "[4/4] detect runtime impact from pulled commits"
      detect_deploy_strategy "$old_head" "$new_head"
      print_changed_files
      ;;
    force-build)
      DEPLOY_STRATEGY="force-build"
      ;;
    no-build)
      DEPLOY_STRATEGY="no-build"
      ;;
    *)
      fail "未知部署模式: $DEPLOY_MODE"
      ;;
  esac
fi

echo "$DEPLOY_STRATEGY"
run_start_script "$DEPLOY_STRATEGY" "${START_ARGS[@]}"

if [[ "$DEPLOY_STRATEGY" == "no-change" || "$DEPLOY_STRATEGY" == "no-runtime-impact" ]]; then
  exit 0
fi

set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

health_url="http://127.0.0.1:${GATEWAY_PORT:-8080}/health"
echo "Health check: $health_url"
for attempt in $(seq 1 10); do
  if curl -fsS "$health_url"; then
    printf '\n'
    exit 0
  fi
  sleep 2
done

fail "健康检查失败: $health_url"
