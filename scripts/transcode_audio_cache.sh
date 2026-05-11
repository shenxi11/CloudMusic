#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'USAGE'
Usage:
  /app/scripts/transcode_audio_cache.sh [--input-dir /data/uploads] [--output-dir /data/transcoded_audio] [--bitrate 320k] [--force]

Purpose:
  Generate MP3 playback cache for large lossless/high-bitrate local audio.
  The output keeps the same relative path as the original and changes the extension to .mp3.
USAGE
}

input_dir="/data/uploads"
output_dir="/data/transcoded_audio"
bitrate="${TRANSCODE_AUDIO_BITRATE:-320k}"
force=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --input-dir)
      input_dir="${2:-}"
      shift 2
      ;;
    --output-dir)
      output_dir="${2:-}"
      shift 2
      ;;
    --bitrate)
      bitrate="${2:-}"
      shift 2
      ;;
    --force)
      force=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Error: unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ -z "$input_dir" || -z "$output_dir" || -z "$bitrate" ]]; then
  echo "Error: input-dir, output-dir and bitrate cannot be empty." >&2
  exit 2
fi
if [[ ! -d "$input_dir" ]]; then
  echo "Error: input directory does not exist: $input_dir" >&2
  exit 1
fi

mkdir -p "$output_dir"

scanned=0
created=0
skipped=0
failed=0

while IFS= read -r -d '' input; do
  scanned=$((scanned + 1))
  rel="${input#"$input_dir"/}"
  base="${rel%.*}"
  output="$output_dir/$base.mp3"

  if [[ -f "$output" && "$force" != "1" ]]; then
    skipped=$((skipped + 1))
    continue
  fi

  mkdir -p "$(dirname "$output")"
  echo "Transcoding: $rel -> ${base}.mp3"
  if ffmpeg -hide_banner -loglevel error -y -i "$input" -vn -map 0:a:0 -codec:a libmp3lame -b:a "$bitrate" "$output"; then
    created=$((created + 1))
  else
    failed=$((failed + 1))
    rm -f "$output"
    echo "Warning: failed to transcode: $rel" >&2
  fi
done < <(find "$input_dir" -type f \( -iname '*.flac' -o -iname '*.dsf' -o -iname '*.ape' -o -iname '*.wav' \) -print0 | sort -z)

echo "Transcode audio cache finished: scanned=$scanned created=$created skipped=$skipped failed=$failed output_dir=$output_dir bitrate=$bitrate"
if [[ "$failed" -gt 0 ]]; then
  exit 1
fi
