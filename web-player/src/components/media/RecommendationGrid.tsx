import { Disc3, Play } from "lucide-react";
import type { RecommendationData } from "../../api";
import type { RecommendScene } from "../../models";
import { guessTitle } from "../../utils";

type RecommendationGridProps = {
  data: RecommendationData | null;
  scene: RecommendScene;
  loading: boolean;
  onSceneChange: (scene: RecommendScene) => void;
  onPlay: (index: number) => void;
};

const sceneOptions: RecommendScene[] = ["home", "radio", "detail"];

export function RecommendationGrid({
  data,
  scene,
  loading,
  onSceneChange,
  onPlay
}: RecommendationGridProps) {
  const items = data?.items || [];

  return (
    <div className="recommend-layout">
      <section className="hero-banner">
        <div className="hero-copy">
          <span className="eyebrow">Discover</span>
          <h1>今日推荐</h1>
          <p>基于你的播放、收藏和最近偏好生成推荐流，当前场景为 {scene}。</p>
          <div className="scene-switcher">
            {sceneOptions.map((option) => (
              <button
                key={option}
                type="button"
                className={`scene-chip ${scene === option ? "is-active" : ""}`}
                onClick={() => onSceneChange(option)}
              >
                {option}
              </button>
            ))}
          </div>
        </div>
        <div className="hero-highlight">
          {items[0] ? (
            <>
              <div
                className="hero-cover"
                style={items[0].cover_art_url ? { backgroundImage: `url(${items[0].cover_art_url})` } : undefined}
              />
              <div className="hero-track">
                <strong>{items[0].title || guessTitle(items[0].path)}</strong>
                <span>{items[0].artist || "未知歌手"}</span>
                <small>{items[0].reason || items[0].source}</small>
                <button type="button" onClick={() => onPlay(0)}>
                  <Play size={16} />
                  <span>播放第一首</span>
                </button>
              </div>
            </>
          ) : (
            <div className="hero-empty">
              <Disc3 size={28} />
              <strong>{loading ? "推荐加载中..." : "暂无推荐"}</strong>
            </div>
          )}
        </div>
      </section>

      <section className="recommend-grid">
        {loading && <div className="empty-panel">推荐数据加载中...</div>}
        {!loading &&
          items.map((item, index) => (
            <article key={`${item.path}_${item.score}`} className="recommend-card">
              <div
                className="recommend-cover"
                style={item.cover_art_url ? { backgroundImage: `url(${item.cover_art_url})` } : undefined}
              >
                <button type="button" className="card-play" onClick={() => onPlay(index)}>
                  <Play size={18} />
                </button>
              </div>
              <div className="recommend-copy">
                <strong>{item.title || guessTitle(item.path)}</strong>
                <span>{item.artist || "未知歌手"}</span>
                <small>{item.source} · {item.reason}</small>
              </div>
            </article>
          ))}
      </section>
    </div>
  );
}
