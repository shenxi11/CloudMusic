#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MIGRATOR_BIN="$SCRIPT_DIR/migrator"
CONFIG_PATH="$SCRIPT_DIR/configs/config.yaml"

if [ ! -x "$MIGRATOR_BIN" ]; then
  echo "错误: 未找到 migrator 可执行文件: $MIGRATOR_BIN"
  echo "请先执行: ./build.sh"
  exit 1
fi

"$MIGRATOR_BIN" -config "$CONFIG_PATH" -service all
