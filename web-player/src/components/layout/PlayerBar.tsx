import {
  Heart,
  ListMusic,
  Pause,
  Play,
  Repeat,
  Shuffle,
  StepBack,
  StepForward,
  X
} from "lucide-react";
import type { PlaybackMode, QueueItem } from "../../models";
import { formatTime, modeLabel } from "../../utils";

type PlayerBarProps = {
  currentTrack: QueueItem | null;
  queueIndex: number;
  queueSize: number;
  playing: boolean;
  playbackMode: PlaybackMode;
  currentSec: number;
  durationSec: number;
  lyricsOpen: boolean;
  mobileSheetOpen: boolean;
  onPrev: () => void;
  onNext: () => void;
  onTogglePlay: () => void;
  onCycleMode: () => void;
  onToggleLyrics: () => void;
  onSeek: (value: number) => void;
  onLikeCurrent: () => void;
  onOpenMobileSheet: () => void;
  onCloseMobileSheet: () => void;
};

function modeIcon(mode: PlaybackMode) {
  if (mode === "single") return <Repeat size={18} />;
  if (mode === "random") return <Shuffle size={18} />;
  return <ListMusic size={18} />;
}

export function PlayerBar({
  currentTrack,
  queueIndex,
  queueSize,
  playing,
  playbackMode,
  currentSec,
  durationSec,
  lyricsOpen,
  mobileSheetOpen,
  onPrev,
  onNext,
  onTogglePlay,
  onCycleMode,
  onToggleLyrics,
  onSeek,
  onLikeCurrent,
  onOpenMobileSheet,
  onCloseMobileSheet
}: PlayerBarProps) {
  const progressValue = durationSec > 0 ? currentSec : 0;
  const rangeMax = durationSec > 0 ? durationSec : 1;

  return (
    <>
      <footer className="player-dock">
        <button
          type="button"
          className="player-compact"
          onClick={() => currentTrack && onOpenMobileSheet()}
          disabled={!currentTrack}
        >
          <div
            className="player-cover"
            style={currentTrack?.coverArtUrl ? { backgroundImage: `url(${currentTrack.coverArtUrl})` } : undefined}
          />
          <div className="player-track-meta">
            <strong>{currentTrack?.title || "未播放"}</strong>
            <span>{currentTrack?.artist || "请选择歌曲"}</span>
          </div>
        </button>

        <div className="player-center">
          <div className="player-controls">
            <button type="button" className="icon-button" onClick={onPrev} disabled={queueIndex <= 0}>
              <StepBack size={18} />
            </button>
            <button type="button" className="play-button" onClick={onTogglePlay} disabled={!currentTrack}>
              {playing ? <Pause size={20} /> : <Play size={20} />}
            </button>
            <button type="button" className="icon-button" onClick={onNext} disabled={queueSize === 0}>
              <StepForward size={18} />
            </button>
          </div>

          <div className="player-progress">
            <span>{formatTime(currentSec)}</span>
            <input
              type="range"
              min={0}
              max={rangeMax}
              step={1}
              value={progressValue}
              onChange={(event) => onSeek(Number(event.target.value))}
              disabled={!currentTrack}
            />
            <span>{formatTime(durationSec)}</span>
          </div>
        </div>

        <div className="player-actions">
          <button type="button" className="icon-button" onClick={onLikeCurrent} disabled={!currentTrack}>
            <Heart size={18} />
          </button>
          <button type="button" className={`text-chip ${lyricsOpen ? "is-active" : ""}`} onClick={onToggleLyrics} disabled={!currentTrack?.lrcUrl}>
            歌词
          </button>
          <button type="button" className="icon-button" onClick={onCycleMode}>
            {modeIcon(playbackMode)}
          </button>
          <span className="player-mode-text">{modeLabel(playbackMode)}</span>
          <span className="player-queue-text">{queueIndex >= 0 ? `${queueIndex + 1}/${queueSize}` : "未入队"}</span>
        </div>
      </footer>

      {mobileSheetOpen && currentTrack && (
        <div className="mobile-player-sheet" onClick={onCloseMobileSheet}>
          <div className="mobile-player-card" onClick={(event) => event.stopPropagation()}>
            <div className="mobile-player-head">
              <strong>正在播放</strong>
              <button type="button" className="icon-button subtle" onClick={onCloseMobileSheet}>
                <X size={16} />
              </button>
            </div>
            <div
              className="mobile-player-cover"
              style={currentTrack.coverArtUrl ? { backgroundImage: `url(${currentTrack.coverArtUrl})` } : undefined}
            />
            <div className="mobile-player-meta">
              <h3>{currentTrack.title}</h3>
              <p>{currentTrack.artist}</p>
            </div>
            <div className="player-progress stacked">
              <input
                type="range"
                min={0}
                max={rangeMax}
                step={1}
                value={progressValue}
                onChange={(event) => onSeek(Number(event.target.value))}
                disabled={!currentTrack}
              />
              <div className="mobile-progress-meta">
                <span>{formatTime(currentSec)}</span>
                <span>{formatTime(durationSec)}</span>
              </div>
            </div>
            <div className="player-controls mobile">
              <button type="button" className="icon-button" onClick={onPrev} disabled={queueIndex <= 0}>
                <StepBack size={18} />
              </button>
              <button type="button" className="play-button" onClick={onTogglePlay}>
                {playing ? <Pause size={20} /> : <Play size={20} />}
              </button>
              <button type="button" className="icon-button" onClick={onNext} disabled={queueSize === 0}>
                <StepForward size={18} />
              </button>
            </div>
            <div className="mobile-player-actions">
              <button type="button" className="text-chip" onClick={onLikeCurrent}>
                喜欢
              </button>
              <button type="button" className={`text-chip ${lyricsOpen ? "is-active" : ""}`} onClick={onToggleLyrics}>
                {lyricsOpen ? "隐藏歌词" : "显示歌词"}
              </button>
              <button type="button" className="text-chip" onClick={onCycleMode}>
                {modeLabel(playbackMode)}
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
