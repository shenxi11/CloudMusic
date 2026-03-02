#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

usage() {
  cat <<'USAGE'
Usage:
  ./scripts/sync_media_db.sh [args]

Args passthrough to mediaindexer:
  -config <path>      配置文件路径（默认 configs/config.yaml）
  -audio-dir <path>   音频目录（默认读取配置 server.upload_dir）
  -video-dir <path>   视频目录（默认读取配置 server.video_dir）
  -skip-video         跳过视频扫描
  -dry-run            只扫描不写库

Example:
  ./scripts/sync_media_db.sh \
    -config configs/config.yaml \
    -audio-dir /home/shen/PycharmProjects/flaskProject/uploads \
    -video-dir /home/shen/PycharmProjects/flaskProject/video
USAGE
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

cd "$ROOT_DIR"

if [[ -x "$ROOT_DIR/media_indexer" ]]; then
  exec "$ROOT_DIR/media_indexer" "$@"
fi

if command -v go >/dev/null 2>&1; then
  exec go run ./cmd/mediaindexer "$@"
fi

echo "Error: 未找到可执行文件 media_indexer，且系统无 go 命令。"
echo "请先执行 ./build.sh，或安装 Go 后重试。"
exit 1

