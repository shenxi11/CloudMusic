import type { LyricLine, PlaybackMode } from "./models";

export function guessTitle(path: string): string {
  const parts = path.split("/");
  const name = parts[parts.length - 1] || path;
  const dot = name.lastIndexOf(".");
  return dot > 0 ? name.slice(0, dot) : name;
}

export function bytesToMB(size: number): string {
  if (!size || Number.isNaN(size)) return "-";
  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
}

export function parseLyricText(raw: string): LyricLine[] {
  const text = raw.trim();
  if (!text) return [];

  try {
    const parsed = JSON.parse(text);
    if (Array.isArray(parsed)) {
      return parsed
        .map((entry) => String(entry).trim())
        .filter(Boolean)
        .map((entry) => ({ timeSec: null, text: entry }));
    }
  } catch {
    // ignore malformed JSON and fallback to line parsing
  }

  const lines = text.split(/\r?\n/).map((line) => line.trim()).filter(Boolean);
  const timeTag = /\[(\d{1,2}):(\d{2})(?:\.(\d{1,3}))?\]/g;
  const out: LyricLine[] = [];

  for (const line of lines) {
    const matches = Array.from(line.matchAll(timeTag));
    const plainText = line.replace(timeTag, "").trim();

    if (matches.length === 0) {
      out.push({ timeSec: null, text: plainText || line });
      continue;
    }

    for (const match of matches) {
      const min = Number(match[1] || "0");
      const sec = Number(match[2] || "0");
      const ms = String(match[3] || "0").padEnd(3, "0").slice(0, 3);
      out.push({
        timeSec: min * 60 + sec + Number(ms) / 1000,
        text: plainText || "♪"
      });
    }
  }

  const timed = out
    .filter((item) => item.timeSec !== null)
    .sort((a, b) => (a.timeSec as number) - (b.timeSec as number));

  return timed.length > 0 ? timed : out;
}

export function findActiveLyricIndex(lines: LyricLine[], currentTime: number): number {
  if (lines.length === 0) return -1;
  if (lines[0].timeSec === null) return -1;

  let left = 0;
  let right = lines.length - 1;
  let answer = -1;

  while (left <= right) {
    const middle = (left + right) >> 1;
    const timeSec = lines[middle].timeSec as number;
    if (timeSec <= currentTime + 0.05) {
      answer = middle;
      left = middle + 1;
    } else {
      right = middle - 1;
    }
  }

  return answer;
}

export function formatTime(sec: number): string {
  if (!Number.isFinite(sec) || sec < 0) return "00:00";
  const total = Math.floor(sec);
  const min = Math.floor(total / 60);
  const second = total % 60;
  return `${String(min).padStart(2, "0")}:${String(second).padStart(2, "0")}`;
}

export function modeLabel(mode: PlaybackMode): string {
  switch (mode) {
    case "single":
      return "单曲循环";
    case "random":
      return "随机播放";
    default:
      return "顺序播放";
  }
}
