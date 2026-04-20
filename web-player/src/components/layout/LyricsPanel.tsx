import { X } from "lucide-react";
import type { LyricLine } from "../../models";

type LyricsPanelProps = {
  title: string;
  loading: boolean;
  lines: LyricLine[];
  activeIndex: number;
  bodyRef: React.RefObject<HTMLDivElement>;
  onClose: () => void;
};

export function LyricsPanel({ title, loading, lines, activeIndex, bodyRef, onClose }: LyricsPanelProps) {
  return (
    <aside className="lyrics-panel">
      <div className="surface-head">
        <div>
          <strong>{title}</strong>
          <small>滚动歌词</small>
        </div>
        <button type="button" className="icon-button subtle" onClick={onClose}>
          <X size={16} />
        </button>
      </div>
      <div className="lyrics-body" ref={bodyRef}>
        {loading && <p className="panel-note">歌词加载中...</p>}
        {!loading && lines.length === 0 && <p className="panel-note">暂无歌词</p>}
        {lines.map((line, index) => (
          <p
            key={`${line.timeSec ?? "plain"}_${line.text}_${index}`}
            className={`lyric-line ${index === activeIndex ? "is-active" : ""}`}
            data-lyric-index={index}
          >
            {line.text}
          </p>
        ))}
      </div>
    </aside>
  );
}
