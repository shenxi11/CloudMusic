export type AppTab = "recommend" | "music" | "video" | "favorites" | "search";

export type RecommendScene = "home" | "radio" | "detail";

export type PlaybackMode = "seq" | "single" | "random";

export type UserSession = {
  account: string;
  username: string;
};

export type RecContext = {
  requestId: string;
  modelVersion: string;
  scene: RecommendScene;
};

export type QueueItem = {
  path: string;
  title: string;
  artist: string;
  album?: string;
  durationSec?: number;
  coverArtUrl?: string;
  streamUrl?: string;
  lrcUrl?: string;
  recContext?: RecContext;
};

export type LyricLine = {
  timeSec: number | null;
  text: string;
};

export type TrackRowItem = {
  id: string;
  path: string;
  title: string;
  artist: string;
  album?: string;
  durationLabel?: string;
  coverArtUrl?: string;
  meta?: string;
};
