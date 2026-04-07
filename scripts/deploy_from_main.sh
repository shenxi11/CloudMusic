#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="$ROOT_DIR/.env.docker"
RENDER_SCRIPT="$ROOT_DIR/scripts/docker/render_config.sh"
DOCKER_SCRIPT="$ROOT_DIR/scripts/docker.sh"

fail() {
  echo "Error: $*" >&2
  exit 1
}

command -v git >/dev/null 2>&1 || fail "git 未安装"
[[ -f "$ENV_FILE" ]] || fail "$ENV_FILE 不存在"
[[ -x "$DOCKER_SCRIPT" ]] || fail "$DOCKER_SCRIPT 不可执行"

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

echo "[1/5] fetch origin"
git -C "$ROOT_DIR" fetch origin

echo "[2/5] checkout main"
git -C "$ROOT_DIR" checkout main

echo "[3/5] pull latest main"
git -C "$ROOT_DIR" pull --ff-only origin main

echo "[4/5] render config"
"$RENDER_SCRIPT" "$ENV_FILE"

echo "[5/5] restart cloudmusic services"
"$DOCKER_SCRIPT" restart

set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

health_url="http://127.0.0.1:${GATEWAY_PORT:-8080}/health"
echo "Health check: $health_url"
curl -fsS "$health_url"
printf '\n'
