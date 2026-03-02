#!/usr/bin/env bash

set -euo pipefail

HOST="${1:?missing host}"
PORT="${2:?missing port}"
TIMEOUT_SEC="${3:-60}"

for ((i=1; i<=TIMEOUT_SEC; i++)); do
  if nc -z "$HOST" "$PORT" >/dev/null 2>&1; then
    exit 0
  fi
  sleep 1
done

echo "timeout waiting for ${HOST}:${PORT} (${TIMEOUT_SEC}s)"
exit 1
