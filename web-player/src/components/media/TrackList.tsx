import { Pause, Play } from "lucide-react";
import type { TrackRowItem } from "../../models";

type TrackListProps = {
  rows: TrackRowItem[];
  loading: boolean;
  emptyTitle: string;
  emptyDescription: string;
  currentTrackPath?: string;
  isPlaying: boolean;
  onPlayRow: (index: number) => void;
  onToggleCurrent: () => void;
};

export function TrackList({
  rows,
  loading,
  emptyTitle,
  emptyDescription,
  currentTrackPath,
  isPlaying,
  onPlayRow,
  onToggleCurrent
}: TrackListProps) {
  if (loading) {
    return <div className="empty-panel">数据加载中...</div>;
  }

  if (rows.length === 0) {
    return (
      <div className="empty-panel">
        <strong>{emptyTitle}</strong>
        <p>{emptyDescription}</p>
      </div>
    );
  }

  return (
    <section className="track-surface">
      <div className="track-head">
        <span>歌曲</span>
        <span>专辑</span>
        <span>时长</span>
        <span>操作</span>
      </div>
      <div className="track-body">
        {rows.map((row, index) => {
          const isCurrent = currentTrackPath === row.path;
          return (
            <article key={row.id} className={`track-row ${isCurrent ? "is-active" : ""}`}>
              <div className="track-main">
                <div className="track-cover" style={row.coverArtUrl ? { backgroundImage: `url(${row.coverArtUrl})` } : undefined} />
                <div className="track-copy">
                  <strong>{row.title}</strong>
                  <span>{row.artist}</span>
                  {row.meta && <small>{row.meta}</small>}
                </div>
              </div>
              <div className="track-album">{row.album || "-"}</div>
              <div className="track-duration">{row.durationLabel || "-"}</div>
              <div className="track-row-actions">
                <button
                  type="button"
                  className="icon-button"
                  onClick={() => {
                    if (isCurrent) {
                      onToggleCurrent();
                      return;
                    }
                    onPlayRow(index);
                  }}
                >
                  {isCurrent && isPlaying ? <Pause size={16} /> : <Play size={16} />}
                </button>
              </div>
            </article>
          );
        })}
      </div>
    </section>
  );
}
