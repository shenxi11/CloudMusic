import { PlayCircle } from "lucide-react";
import type { VideoFile } from "../../api";
import { bytesToMB, guessTitle } from "../../utils";

type VideoListProps = {
  items: VideoFile[];
  loading: boolean;
  onPlay: (path: string, title: string) => void;
};

export function VideoList({ items, loading, onPlay }: VideoListProps) {
  if (loading) {
    return <div className="empty-panel">视频数据加载中...</div>;
  }

  if (items.length === 0) {
    return (
      <div className="empty-panel">
        <strong>暂无视频</strong>
        <p>服务端视频目录为空时，这里会显示空态。</p>
      </div>
    );
  }

  return (
    <section className="video-grid">
      {items.map((item) => {
        const title = item.name || guessTitle(item.path);
        return (
          <article key={item.path} className="video-card">
            <div className="video-thumb">
              <PlayCircle size={30} />
            </div>
            <div className="video-copy">
              <strong>{title}</strong>
              <span>{bytesToMB(item.size)}</span>
            </div>
            <button type="button" className="text-button" onClick={() => onPlay(item.path, title)}>
              播放视频
            </button>
          </article>
        );
      })}
    </section>
  );
}
