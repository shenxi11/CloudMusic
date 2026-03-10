import { FormEvent, SyntheticEvent, useEffect, useMemo, useRef, useState } from "react";
import {
  addFavorite,
  addPlayHistory,
  FavoriteItem,
  getMusicDetail,
  getVideoStream,
  listFavorites,
  listMusic,
  listRecommendations,
  listVideos,
  login,
  MusicFileItem,
  pingServer,
  postRecommendationFeedback,
  RecommendationData,
  registerUser,
  searchMusic,
  VideoFile
} from "./api";

type AppTab = "recommend" | "music" | "search" | "video" | "favorites";
type RecommendScene = "home" | "radio" | "detail";
type PlaybackMode = "seq" | "single" | "random";

type UserSession = {
  account: string;
  username: string;
};

type RecContext = {
  requestId: string;
  modelVersion: string;
  scene: RecommendScene;
};

type QueueItem = {
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

type LyricLine = {
  timeSec: number | null;
  text: string;
};

const STORAGE_SERVER = "cloudmusic_web_server";
const STORAGE_USER = "cloudmusic_web_user";

function guessTitle(path: string): string {
  const parts = path.split("/");
  const name = parts[parts.length - 1] || path;
  const dot = name.lastIndexOf(".");
  return dot > 0 ? name.slice(0, dot) : name;
}

function bytesToMB(size: number): string {
  if (!size || Number.isNaN(size)) return "-";
  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
}

function parseLyricText(raw: string): LyricLine[] {
  const text = raw.trim();
  if (!text) return [];

  try {
    const parsed = JSON.parse(text);
    if (Array.isArray(parsed)) {
      return parsed
        .map((x) => String(x).trim())
        .filter(Boolean)
        .map((x) => ({ timeSec: null, text: x }));
    }
  } catch {
    // ignore
  }

  const lines = text.split(/\r?\n/).map((l) => l.trim()).filter(Boolean);
  const timeTag = /\[(\d{1,2}):(\d{2})(?:\.(\d{1,3}))?\]/g;
  const out: LyricLine[] = [];

  for (const line of lines) {
    const matches = Array.from(line.matchAll(timeTag));
    const pure = line.replace(timeTag, "").trim();
    if (matches.length === 0) {
      out.push({ timeSec: null, text: pure || line });
      continue;
    }

    for (const m of matches) {
      const min = Number(m[1] || "0");
      const sec = Number(m[2] || "0");
      const ms = String(m[3] || "0").padEnd(3, "0").slice(0, 3);
      const frac = Number(ms) / 1000;
      out.push({ timeSec: min * 60 + sec + frac, text: pure || "♪" });
    }
  }

  const timed = out.filter((x) => x.timeSec !== null).sort((a, b) => (a.timeSec as number) - (b.timeSec as number));
  if (timed.length > 0) return timed;
  return out;
}

function findActiveLyricIndex(lines: LyricLine[], currentTime: number): number {
  if (lines.length === 0) return -1;
  if (lines[0].timeSec === null) return -1;
  let lo = 0;
  let hi = lines.length - 1;
  let ans = -1;
  while (lo <= hi) {
    const mid = (lo + hi) >> 1;
    const t = lines[mid].timeSec as number;
    if (t <= currentTime + 0.05) {
      ans = mid;
      lo = mid + 1;
    } else {
      hi = mid - 1;
    }
  }
  return ans;
}

function modeLabel(mode: PlaybackMode): string {
  switch (mode) {
    case "single":
      return "单曲循环";
    case "random":
      return "随机播放";
    default:
      return "顺序播放";
  }
}

function formatTime(sec: number): string {
  if (!Number.isFinite(sec) || sec < 0) return "00:00";
  const total = Math.floor(sec);
  const min = Math.floor(total / 60);
  const s = total % 60;
  return `${String(min).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
}

export default function App() {
  const [isMobile, setIsMobile] = useState<boolean>(() => {
    if (typeof window === "undefined") return false;
    return window.matchMedia("(max-width: 900px)").matches;
  });
  const [serverInput, setServerInput] = useState<string>(() => localStorage.getItem(STORAGE_SERVER) || "http://127.0.0.1:8080");
  const [serverUrl, setServerUrl] = useState<string>("");
  const [connectErr, setConnectErr] = useState<string>("");
  const [connecting, setConnecting] = useState(false);

  const [session, setSession] = useState<UserSession | null>(() => {
    const raw = localStorage.getItem(STORAGE_USER);
    if (!raw) return null;
    try {
      const parsed = JSON.parse(raw) as UserSession;
      if (!parsed.account) return null;
      return parsed;
    } catch {
      return null;
    }
  });

  const [authMode, setAuthMode] = useState<"login" | "register">("login");
  const [account, setAccount] = useState("");
  const [password, setPassword] = useState("");
  const [username, setUsername] = useState("");
  const [authLoading, setAuthLoading] = useState(false);
  const [authErr, setAuthErr] = useState("");

  const [tab, setTab] = useState<AppTab>("recommend");
  const [loadingData, setLoadingData] = useState(false);
  const [dataErr, setDataErr] = useState("");

  const [recommendScene, setRecommendScene] = useState<RecommendScene>("home");
  const [recommendData, setRecommendData] = useState<RecommendationData | null>(null);
  const [musicList, setMusicList] = useState<MusicFileItem[]>([]);
  const [searchKeyword, setSearchKeyword] = useState("");
  const [searchLoading, setSearchLoading] = useState(false);
  const [searchList, setSearchList] = useState<MusicFileItem[]>([]);
  const [videoList, setVideoList] = useState<VideoFile[]>([]);
  const [favoriteList, setFavoriteList] = useState<FavoriteItem[]>([]);

  const [queueItems, setQueueItems] = useState<QueueItem[]>([]);
  const [queueIndex, setQueueIndex] = useState<number>(-1);
  const [currentTrack, setCurrentTrack] = useState<QueueItem | null>(null);
  const [playbackMode, setPlaybackMode] = useState<PlaybackMode>("seq");
  const [playing, setPlaying] = useState(false);
  const [playErr, setPlayErr] = useState("");
  const [audioCurrentSec, setAudioCurrentSec] = useState(0);
  const [audioDurationSec, setAudioDurationSec] = useState(0);
  const [mobilePlayerOpen, setMobilePlayerOpen] = useState(false);

  const [lyricsOpen, setLyricsOpen] = useState(false);
  const [lyricsLoading, setLyricsLoading] = useState(false);
  const [lyricLines, setLyricLines] = useState<LyricLine[]>([]);
  const [activeLyricIndex, setActiveLyricIndex] = useState(-1);

  const [activeVideo, setActiveVideo] = useState<{ title: string; url: string } | null>(null);

  const audioRef = useRef<HTMLAudioElement | null>(null);
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const lyricBodyRef = useRef<HTMLDivElement | null>(null);

  const displayUser = useMemo(() => {
    if (!session) return "未登录";
    return session.username ? `${session.username} (${session.account})` : session.account;
  }, [session]);

  useEffect(() => {
    if (!serverInput) return;
    void tryConnect(serverInput, true);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (typeof window === "undefined") return undefined;
    const media = window.matchMedia("(max-width: 900px)");
    const onChange = () => setIsMobile(media.matches);
    onChange();
    if (typeof media.addEventListener === "function") {
      media.addEventListener("change", onChange);
      return () => media.removeEventListener("change", onChange);
    }
    media.addListener(onChange);
    return () => media.removeListener(onChange);
  }, []);

  useEffect(() => {
    if (!serverUrl || !session) return;
    void refreshData(serverUrl, session, recommendScene);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [serverUrl, session?.account, recommendScene]);

  useEffect(() => {
    if (!currentTrack?.lrcUrl) {
      setLyricLines([]);
      setActiveLyricIndex(-1);
      return;
    }
    void loadLyrics(currentTrack.lrcUrl);
  }, [currentTrack?.lrcUrl]);

  useEffect(() => {
    setAudioCurrentSec(0);
    setAudioDurationSec(0);
  }, [currentTrack?.streamUrl]);

  useEffect(() => {
    if (!lyricsOpen || activeLyricIndex < 0 || !lyricBodyRef.current) return;
    const el = lyricBodyRef.current.querySelector<HTMLElement>(`[data-lyric-index="${activeLyricIndex}"]`);
    if (!el) return;
    el.scrollIntoView({ block: "center", behavior: "smooth" });
  }, [activeLyricIndex, lyricsOpen]);

  useEffect(() => {
    if (!isMobile) setMobilePlayerOpen(false);
  }, [isMobile]);

  useEffect(() => {
    const videoEl = videoRef.current;
    if (!videoEl || !activeVideo) return;

    let cancelled = false;
    let cleanup = () => {};

    const boot = async () => {
      const src = activeVideo.url;
      if (src.toLowerCase().includes(".m3u8")) {
        const mod = await import("hls.js");
        if (cancelled) return;
        const Hls = mod.default;
        if (Hls.isSupported()) {
          const hls = new Hls({ enableWorker: true, lowLatencyMode: true });
          hls.loadSource(src);
          hls.attachMedia(videoEl);
          cleanup = () => hls.destroy();
        } else {
          videoEl.src = src;
        }
      } else {
        videoEl.src = src;
      }
      void videoEl.play().catch(() => undefined);
    };

    void boot();
    return () => {
      cancelled = true;
      cleanup();
      videoEl.pause();
      videoEl.removeAttribute("src");
      videoEl.load();
    };
  }, [activeVideo]);

  async function tryConnect(input: string, silent = false) {
    const normalized = input.trim().replace(/\/+$/, "");
    if (!normalized) {
      if (!silent) setConnectErr("请输入服务器地址，例如 http://127.0.0.1:8080");
      return;
    }
    setConnecting(true);
    setConnectErr("");
    try {
      await pingServer(normalized);
      setServerUrl(normalized);
      localStorage.setItem(STORAGE_SERVER, normalized);
      setServerInput(normalized);
    } catch (e) {
      if (!silent) setConnectErr((e as Error).message || "连接失败");
      setServerUrl("");
    } finally {
      setConnecting(false);
    }
  }

  async function onAuthSubmit(e: FormEvent) {
    e.preventDefault();
    if (!serverUrl) {
      setAuthErr("请先连接服务器");
      return;
    }
    if (!account.trim() || !password.trim()) {
      setAuthErr("账号和密码不能为空");
      return;
    }

    setAuthLoading(true);
    setAuthErr("");
    try {
      if (authMode === "register") {
        if (!username.trim()) throw new Error("注册模式下用户名不能为空");
        await registerUser(serverUrl, account.trim(), password, username.trim());
      }
      const ret = await login(serverUrl, account.trim(), password);
      if (!ret.success_bool) throw new Error("账号或密码错误");

      const nextSession: UserSession = {
        account: account.trim(),
        username: ret.username || username || account.trim()
      };
      setSession(nextSession);
      localStorage.setItem(STORAGE_USER, JSON.stringify(nextSession));
      setPassword("");
    } catch (e) {
      setAuthErr((e as Error).message || "认证失败");
    } finally {
      setAuthLoading(false);
    }
  }

  async function refreshData(base: string, user: UserSession, scene: RecommendScene) {
    setLoadingData(true);
    setDataErr("");
    try {
      const [recRet, musicRet, videoRet, favRet] = await Promise.allSettled([
        listRecommendations(base, user.account, scene),
        listMusic(base),
        listVideos(base),
        listFavorites(base, user.account)
      ]);

      if (recRet.status === "fulfilled") setRecommendData(recRet.value);
      if (musicRet.status === "fulfilled") setMusicList(musicRet.value);
      if (videoRet.status === "fulfilled") setVideoList(videoRet.value);
      if (favRet.status === "fulfilled") setFavoriteList(favRet.value);

      const errors = [recRet, musicRet, videoRet, favRet]
        .filter((x) => x.status === "rejected")
        .map((x) => (x as PromiseRejectedResult).reason?.message || "数据加载失败");
      if (errors.length > 0) {
        setDataErr(errors.join(" | "));
      }
    } finally {
      setLoadingData(false);
    }
  }

  async function searchMusicList() {
    if (!serverUrl || !searchKeyword.trim()) return;
    setSearchLoading(true);
    setDataErr("");
    try {
      const ret = await searchMusic(serverUrl, searchKeyword.trim());
      setSearchList(ret);
      setTab("search");
    } catch (e) {
      setDataErr((e as Error).message || "搜索失败");
    } finally {
      setSearchLoading(false);
    }
  }

  async function buildQueueAndPlay(items: QueueItem[], startIndex: number) {
    if (!session || !serverUrl || items.length === 0) return;
    setQueueItems(items);
    await playQueueIndex(items, startIndex);
  }

  async function playQueueIndex(items: QueueItem[], index: number, emitSkip = false) {
    if (!session || !serverUrl) return;
    if (index < 0 || index >= items.length) return;

    const prev = currentTrack;
    if (emitSkip && prev?.recContext) {
      void postRecommendationFeedback(serverUrl, {
        user_id: session.account,
        song_id: prev.path,
        event_type: "skip",
        request_id: prev.recContext.requestId,
        model_version: prev.recContext.modelVersion,
        scene: prev.recContext.scene
      }).catch(() => undefined);
    }

    setPlayErr("");
    try {
      const candidate = items[index];
      let track = candidate;
      if (!candidate.streamUrl) {
        const detail = await getMusicDetail(serverUrl, candidate.path);
        track = {
          ...candidate,
          streamUrl: detail.stream_url,
          lrcUrl: detail.lrc_url,
          coverArtUrl: detail.album_cover_url,
          album: detail.album || candidate.album,
          title: detail.title || candidate.title,
          artist: detail.artist || candidate.artist,
          durationSec: detail.duration || candidate.durationSec
        };
      }

      setCurrentTrack(track);
      setQueueIndex(index);

      await addPlayHistory(serverUrl, session.account, {
        music_path: track.path,
        music_title: track.title,
        artist: track.artist,
        album: track.album,
        duration_sec: track.durationSec,
        is_local: false
      }).catch(() => undefined);

      if (track.recContext) {
        await postRecommendationFeedback(serverUrl, {
          user_id: session.account,
          song_id: track.path,
          event_type: "play",
          request_id: track.recContext.requestId,
          model_version: track.recContext.modelVersion,
          scene: track.recContext.scene
        }).catch(() => undefined);
      }
    } catch (e) {
      setPlayErr((e as Error).message || "播放失败");
    }
  }

  async function playRecommendationAt(index: number) {
    if (!recommendData) return;
    const queue = recommendData.items.map((item) => ({
      path: item.path,
      title: item.title || guessTitle(item.path),
      artist: item.artist || "未知歌手",
      album: item.album,
      durationSec: item.duration_sec,
      coverArtUrl: item.cover_art_url,
      streamUrl: item.stream_url,
      lrcUrl: item.lrc_url,
      recContext: {
        requestId: recommendData.request_id,
        modelVersion: recommendData.model_version,
        scene: recommendScene
      }
    }));
    await buildQueueAndPlay(queue, index);
  }

  async function playMusicAt(list: MusicFileItem[], index: number) {
    const queue = list.map((item) => ({
      path: item.path,
      title: guessTitle(item.path),
      artist: item.artist || "未知歌手"
    }));
    await buildQueueAndPlay(queue, index);
  }

  async function playFavoriteAt(list: FavoriteItem[], index: number) {
    const queue = list.map((item) => ({
      path: item.path,
      title: item.title || guessTitle(item.path),
      artist: item.artist || "未知歌手",
      album: item.album,
      coverArtUrl: item.cover_art_url
    }));
    await buildQueueAndPlay(queue, index);
  }

  async function onLikeCurrent(track: QueueItem) {
    if (!session || !serverUrl) return;
    try {
      await addFavorite(serverUrl, session.account, {
        music_path: track.path,
        music_title: track.title,
        artist: track.artist,
        duration_sec: track.durationSec,
        is_local: false
      });
      const fav = await listFavorites(serverUrl, session.account);
      setFavoriteList(fav);
      if (track.recContext) {
        await postRecommendationFeedback(serverUrl, {
          user_id: session.account,
          song_id: track.path,
          event_type: "like",
          request_id: track.recContext.requestId,
          model_version: track.recContext.modelVersion,
          scene: track.recContext.scene
        }).catch(() => undefined);
      }
    } catch (e) {
      setPlayErr((e as Error).message || "收藏失败");
    }
  }

  async function loadLyrics(url: string) {
    setLyricsLoading(true);
    setActiveLyricIndex(-1);
    try {
      const resp = await fetch(url);
      const text = await resp.text();
      setLyricLines(parseLyricText(text));
    } catch {
      setLyricLines([]);
    } finally {
      setLyricsLoading(false);
    }
  }

  async function playVideo(path: string, title: string) {
    if (!serverUrl) return;
    try {
      const ret = await getVideoStream(serverUrl, path);
      setActiveVideo({ title, url: ret.url });
    } catch (e) {
      setDataErr((e as Error).message || "加载视频失败");
    }
  }

  function randomNextIndex(current: number, total: number): number {
    if (total <= 1) return current;
    let next = current;
    while (next === current) {
      next = Math.floor(Math.random() * total);
    }
    return next;
  }

  function nextIndexByMode(): number {
    if (queueItems.length === 0 || queueIndex < 0) return -1;
    if (playbackMode === "single") return queueIndex;
    if (playbackMode === "random") return randomNextIndex(queueIndex, queueItems.length);
    const next = queueIndex + 1;
    return next < queueItems.length ? next : -1;
  }

  function playPrev() {
    if (queueItems.length === 0 || queueIndex < 0) return;
    const prev = queueIndex - 1;
    if (prev < 0) return;
    void playQueueIndex(queueItems, prev, true);
  }

  function playNext() {
    if (queueItems.length === 0 || queueIndex < 0) return;
    let next = queueIndex + 1;
    if (playbackMode === "random") {
      next = randomNextIndex(queueIndex, queueItems.length);
    }
    if (next < 0 || next >= queueItems.length) return;
    void playQueueIndex(queueItems, next, true);
  }

  function cyclePlaybackMode() {
    setPlaybackMode((prev) => {
      if (prev === "seq") return "single";
      if (prev === "single") return "random";
      return "seq";
    });
  }

  function togglePlay() {
    const el = audioRef.current;
    if (!el) return;
    if (playing) {
      el.pause();
    } else {
      void el.play().catch(() => undefined);
    }
  }

  function seekAudio(value: number) {
    const el = audioRef.current;
    if (!el || !Number.isFinite(audioDurationSec) || audioDurationSec <= 0) return;
    const next = Math.max(0, Math.min(value, audioDurationSec));
    el.currentTime = next;
    setAudioCurrentSec(next);
  }

  function handleAudioLoadedMeta(e: SyntheticEvent<HTMLAudioElement>) {
    const el = e.currentTarget;
    setAudioDurationSec(Number.isFinite(el.duration) ? el.duration : 0);
  }

  function handleAudioTimeUpdate(e: SyntheticEvent<HTMLAudioElement>) {
    const t = e.currentTarget.currentTime;
    setAudioCurrentSec(t);
    setActiveLyricIndex(findActiveLyricIndex(lyricLines, t));
  }

  function handleAudioEnded() {
    setPlaying(false);
    if (serverUrl && session && currentTrack?.recContext) {
      void postRecommendationFeedback(serverUrl, {
        user_id: session.account,
        song_id: currentTrack.path,
        event_type: "finish",
        request_id: currentTrack.recContext.requestId,
        model_version: currentTrack.recContext.modelVersion,
        scene: currentTrack.recContext.scene
      }).catch(() => undefined);
    }
    const next = nextIndexByMode();
    if (next >= 0) {
      void playQueueIndex(queueItems, next);
    }
  }

  function logout() {
    setSession(null);
    setRecommendData(null);
    setFavoriteList([]);
    setCurrentTrack(null);
    setQueueItems([]);
    setQueueIndex(-1);
    setMobilePlayerOpen(false);
    localStorage.removeItem(STORAGE_USER);
  }

  if (!serverUrl) {
    return (
      <div className="connect-page">
        <div className="connect-card">
          <h1>CloudMusic Web</h1>
          <p>先连接服务器，再进入音视频平台</p>
          <input value={serverInput} onChange={(e) => setServerInput(e.target.value)} placeholder="http://127.0.0.1:8080" />
          <button onClick={() => void tryConnect(serverInput)} disabled={connecting}>
            {connecting ? "连接中..." : "连接服务器"}
          </button>
          {connectErr && <div className="err">{connectErr}</div>}
        </div>
      </div>
    );
  }

  if (!session) {
    return (
      <div className="connect-page">
        <form className="connect-card" onSubmit={onAuthSubmit}>
          <h1>{authMode === "login" ? "用户登录" : "用户注册"}</h1>
          <p>当前服务器：{serverUrl}</p>
          <input value={account} onChange={(e) => setAccount(e.target.value)} placeholder="账号" />
          <input value={password} type="password" onChange={(e) => setPassword(e.target.value)} placeholder="密码" />
          {authMode === "register" && <input value={username} onChange={(e) => setUsername(e.target.value)} placeholder="用户名" />}
          <button type="submit" disabled={authLoading}>
            {authLoading ? "提交中..." : authMode === "login" ? "登录" : "注册并登录"}
          </button>
          <button
            type="button"
            className="ghost"
            onClick={() => {
              setAuthMode(authMode === "login" ? "register" : "login");
              setAuthErr("");
            }}
          >
            {authMode === "login" ? "没有账号？去注册" : "已有账号？去登录"}
          </button>
          <button type="button" className="ghost" onClick={() => setServerUrl("")}>切换服务器</button>
          {authErr && <div className="err">{authErr}</div>}
        </form>
      </div>
    );
  }

  const pageTitle =
    tab === "recommend" ? "为你推荐" :
    tab === "music" ? "音乐库" :
    tab === "search" ? `搜索结果（${searchList.length}）` :
    tab === "video" ? "视频库" :
    "我的喜欢";

  const pageSubtitle =
    tab === "recommend" ? "结合你的历史行为和收藏偏好" :
    tab === "music" ? `已收录 ${musicList.length} 首歌曲` :
    tab === "search" ? "按歌名、歌手快速定位曲目" :
    tab === "video" ? `共 ${videoList.length} 个视频` :
    `已收藏 ${favoriteList.length} 首歌曲`;

  const progressPct = audioDurationSec > 0 ? Math.min(100, (audioCurrentSec / audioDurationSec) * 100) : 0;

  return (
    <div className={`app-shell ${isMobile ? "is-mobile" : "is-desktop"}`}>
      <aside className="side-nav">
        <div className="brand">CloudMusic</div>
        <div className="user-box">
          <div className="avatar">{displayUser.slice(0, 1).toUpperCase()}</div>
          <div>
            <strong>{displayUser}</strong>
            <p>{serverUrl}</p>
          </div>
        </div>

        <div className="search-inline">
          <input value={searchKeyword} onChange={(e) => setSearchKeyword(e.target.value)} placeholder="搜索歌曲/歌手" />
          <button onClick={() => void searchMusicList()} disabled={searchLoading || !searchKeyword.trim()}>
            {searchLoading ? "搜索中" : "搜索"}
          </button>
        </div>

        <div className="nav-list">
          <button className={tab === "recommend" ? "active" : ""} onClick={() => setTab("recommend")}>推荐</button>
          <button className={tab === "music" ? "active" : ""} onClick={() => setTab("music")}>音乐库</button>
          <button className={tab === "search" ? "active" : ""} onClick={() => setTab("search")}>搜索结果</button>
          <button className={tab === "video" ? "active" : ""} onClick={() => setTab("video")}>视频库</button>
          <button className={tab === "favorites" ? "active" : ""} onClick={() => setTab("favorites")}>我的喜欢</button>
        </div>

        <div className="side-footer">
          <button className="ghost" onClick={() => void refreshData(serverUrl, session, recommendScene)} disabled={loadingData}>
            {loadingData ? "刷新中..." : "刷新数据"}
          </button>
          <button className="ghost" onClick={logout}>退出登录</button>
        </div>
      </aside>

      <main className="main-panel">
        <header className="main-header">
          <div>
            <h2>{pageTitle}</h2>
            <p>{pageSubtitle}</p>
          </div>
          {dataErr && <div className="err">{dataErr}</div>}
        </header>

        {tab === "recommend" && (
          <>
            <div className="recommend-tools">
              <span>推荐场景</span>
              <button className={recommendScene === "home" ? "active" : "ghost"} onClick={() => setRecommendScene("home")}>home</button>
              <button className={recommendScene === "radio" ? "active" : "ghost"} onClick={() => setRecommendScene("radio")}>radio</button>
              <button className={recommendScene === "detail" ? "active" : "ghost"} onClick={() => setRecommendScene("detail")}>detail</button>
            </div>
            <section className="card-grid">
              {(recommendData?.items || []).map((item, idx) => (
                <article key={`${item.path}_${item.score}`} className="media-card">
                  <div className="media-cover" style={{ backgroundImage: item.cover_art_url ? `url(${item.cover_art_url})` : undefined }} />
                  <h3>{item.title || guessTitle(item.path)}</h3>
                  <p>{item.artist || "未知歌手"}</p>
                  <div className="meta">{item.source} · {item.reason} · score {item.score.toFixed(3)}</div>
                  <button onClick={() => void playRecommendationAt(idx)}>播放</button>
                </article>
              ))}
            </section>
          </>
        )}

        {tab === "music" && (
          isMobile ? (
            <section className="mobile-list">
              {musicList.map((item, idx) => (
                <article className="mobile-list-item" key={item.path}>
                  <div className="mobile-item-main">
                    <strong>{guessTitle(item.path)}</strong>
                    <span>{item.artist || "未知歌手"}</span>
                  </div>
                  <div className="mobile-item-side">
                    <small>{item.duration || "-"}</small>
                    <button onClick={() => void playMusicAt(musicList, idx)}>播放</button>
                  </div>
                </article>
              ))}
            </section>
          ) : (
            <section className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>标题</th>
                    <th>歌手</th>
                    <th>时长</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {musicList.map((item, idx) => (
                    <tr key={item.path}>
                      <td>{guessTitle(item.path)}</td>
                      <td>{item.artist || "未知歌手"}</td>
                      <td>{item.duration || "-"}</td>
                      <td><button onClick={() => void playMusicAt(musicList, idx)}>播放</button></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </section>
          )
        )}

        {tab === "search" && (
          isMobile ? (
            <section className="mobile-list">
              {searchList.map((item, idx) => (
                <article className="mobile-list-item" key={`${item.path}_${idx}`}>
                  <div className="mobile-item-main">
                    <strong>{guessTitle(item.path)}</strong>
                    <span>{item.artist || "未知歌手"}</span>
                  </div>
                  <div className="mobile-item-side">
                    <small>{item.duration || "-"}</small>
                    <button onClick={() => void playMusicAt(searchList, idx)}>播放</button>
                  </div>
                </article>
              ))}
            </section>
          ) : (
            <section className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>标题</th>
                    <th>歌手</th>
                    <th>时长</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {searchList.map((item, idx) => (
                    <tr key={`${item.path}_${idx}`}>
                      <td>{guessTitle(item.path)}</td>
                      <td>{item.artist || "未知歌手"}</td>
                      <td>{item.duration || "-"}</td>
                      <td><button onClick={() => void playMusicAt(searchList, idx)}>播放</button></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </section>
          )
        )}

        {tab === "video" && (
          isMobile ? (
            <section className="mobile-list">
              {videoList.map((item) => (
                <article className="mobile-list-item" key={item.path}>
                  <div className="mobile-item-main">
                    <strong>{item.name || guessTitle(item.path)}</strong>
                    <span>大小：{bytesToMB(item.size)}</span>
                  </div>
                  <div className="mobile-item-side">
                    <button onClick={() => void playVideo(item.path, item.name)}>播放</button>
                  </div>
                </article>
              ))}
            </section>
          ) : (
            <section className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>文件名</th>
                    <th>大小</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {videoList.map((item) => (
                    <tr key={item.path}>
                      <td>{item.name || guessTitle(item.path)}</td>
                      <td>{bytesToMB(item.size)}</td>
                      <td><button onClick={() => void playVideo(item.path, item.name)}>播放</button></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </section>
          )
        )}

        {tab === "favorites" && (
          isMobile ? (
            <section className="mobile-list">
              {favoriteList.map((item, idx) => (
                <article className="mobile-list-item" key={item.path}>
                  <div className="mobile-item-main">
                    <strong>{item.title || guessTitle(item.path)}</strong>
                    <span>{item.artist || "未知歌手"}</span>
                    <small>{item.album || "-"}</small>
                  </div>
                  <div className="mobile-item-side">
                    <button onClick={() => void playFavoriteAt(favoriteList, idx)}>播放</button>
                  </div>
                </article>
              ))}
            </section>
          ) : (
            <section className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>标题</th>
                    <th>歌手</th>
                    <th>专辑</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {favoriteList.map((item, idx) => (
                    <tr key={item.path}>
                      <td>{item.title || guessTitle(item.path)}</td>
                      <td>{item.artist || "未知歌手"}</td>
                      <td>{item.album || "-"}</td>
                      <td><button onClick={() => void playFavoriteAt(favoriteList, idx)}>播放</button></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </section>
          )
        )}
      </main>

      <footer className={`player-bar ${isMobile ? "mobile-player" : ""}`}>
        {isMobile ? (
          <>
            <div className="mobile-player-top" onClick={() => currentTrack && setMobilePlayerOpen(true)}>
              <div
                className="track-thumb"
                style={{ backgroundImage: currentTrack?.coverArtUrl ? `url(${currentTrack.coverArtUrl})` : undefined }}
              />
              <div className="track-meta">
                <strong>{currentTrack?.title || "未播放"}</strong>
                <span>{currentTrack?.artist || "请选择歌曲"}</span>
                <small>队列：{queueIndex >= 0 ? `${queueIndex + 1}/${queueItems.length}` : "-"}</small>
              </div>
              <div className="mobile-mini-actions">
                <button disabled={!currentTrack} onClick={(e) => { e.stopPropagation(); togglePlay(); }}>
                  {playing ? "暂停" : "播放"}
                </button>
              </div>
            </div>
            <div className="mobile-progress">
              <span>{formatTime(audioCurrentSec)}</span>
              <input
                type="range"
                min={0}
                max={audioDurationSec > 0 ? audioDurationSec : 1}
                step={1}
                value={audioDurationSec > 0 ? audioCurrentSec : 0}
                onChange={(e) => seekAudio(Number(e.target.value))}
                disabled={!currentTrack}
              />
              <span>{formatTime(audioDurationSec)}</span>
            </div>
          </>
        ) : (
          <div className="track-meta">
            <strong>{currentTrack?.title || "未播放"}</strong>
            <span>{currentTrack?.artist || "请选择歌曲"}</span>
            <small>队列：{queueIndex >= 0 ? `${queueIndex + 1}/${queueItems.length}` : "-"}</small>
          </div>
        )}

        <audio
          ref={audioRef}
          className={`audio-core ${isMobile ? "mobile-hidden-audio" : ""}`}
          controls={!isMobile}
          autoPlay
          src={currentTrack?.streamUrl}
          onPlay={() => setPlaying(true)}
          onPause={() => setPlaying(false)}
          onLoadedMetadata={handleAudioLoadedMeta}
          onDurationChange={handleAudioLoadedMeta}
          onTimeUpdate={handleAudioTimeUpdate}
          onEnded={handleAudioEnded}
        />

        <div className="player-actions">
          <button disabled={queueIndex <= 0} onClick={playPrev}>上一首</button>
          <button disabled={!currentTrack} onClick={togglePlay}>{playing ? "暂停" : "播放"}</button>
          <button disabled={queueItems.length === 0} onClick={playNext}>下一首</button>
          <button disabled={!currentTrack} onClick={() => currentTrack && void onLikeCurrent(currentTrack)}>喜欢</button>
          <button className="ghost" onClick={cyclePlaybackMode}>模式：{modeLabel(playbackMode)}</button>
          <button className="ghost" onClick={() => setLyricsOpen(!lyricsOpen)} disabled={!currentTrack?.lrcUrl}>
            {lyricsOpen ? "隐藏歌词" : "歌词"}
          </button>
        </div>
        {playErr && <div className="err">{playErr}</div>}
      </footer>

      {isMobile && mobilePlayerOpen && currentTrack && (
        <div className="mobile-now-playing" onClick={() => setMobilePlayerOpen(false)}>
          <div className="mobile-now-card" onClick={(e) => e.stopPropagation()}>
            <div className="mobile-now-head">
              <strong>正在播放</strong>
              <button className="ghost" onClick={() => setMobilePlayerOpen(false)}>关闭</button>
            </div>
            <div
              className="mobile-now-cover"
              style={{ backgroundImage: currentTrack.coverArtUrl ? `url(${currentTrack.coverArtUrl})` : undefined }}
            />
            <h3>{currentTrack.title}</h3>
            <p>{currentTrack.artist}</p>
            <div className="mobile-now-progress">
              <div className="mobile-now-progress-track">
                <div style={{ width: `${progressPct}%` }} />
              </div>
              <div>
                <span>{formatTime(audioCurrentSec)}</span>
                <span>{formatTime(audioDurationSec)}</span>
              </div>
            </div>
            <div className="mobile-now-controls">
              <button disabled={queueIndex <= 0} onClick={playPrev}>上一首</button>
              <button disabled={!currentTrack} onClick={togglePlay}>{playing ? "暂停" : "播放"}</button>
              <button disabled={queueItems.length === 0} onClick={playNext}>下一首</button>
            </div>
            <div className="mobile-now-actions">
              <button disabled={!currentTrack} onClick={() => currentTrack && void onLikeCurrent(currentTrack)}>喜欢</button>
              <button className="ghost" onClick={cyclePlaybackMode}>{modeLabel(playbackMode)}</button>
              <button className="ghost" onClick={() => setLyricsOpen(!lyricsOpen)} disabled={!currentTrack?.lrcUrl}>
                {lyricsOpen ? "隐藏歌词" : "歌词"}
              </button>
            </div>
          </div>
        </div>
      )}

      {lyricsOpen && (
        <aside className="lyrics-panel">
          <div className="lyrics-head">
            <strong>{currentTrack?.title || "歌词"}</strong>
            <button className="ghost" onClick={() => setLyricsOpen(false)}>关闭</button>
          </div>
          <div className="lyrics-body" ref={lyricBodyRef}>
            {lyricsLoading && <p>歌词加载中...</p>}
            {!lyricsLoading && lyricLines.length === 0 && <p>暂无歌词</p>}
            {lyricLines.map((line, idx) => (
              <p
                key={`${line.timeSec ?? "n"}_${line.text}_${idx}`}
                data-lyric-index={idx}
                className={`lyrics-line ${idx === activeLyricIndex ? "active" : ""}`}
              >
                {line.text}
              </p>
            ))}
          </div>
        </aside>
      )}

      {activeVideo && (
        <div className="video-modal" onClick={() => setActiveVideo(null)}>
          <div className="video-wrap" onClick={(e) => e.stopPropagation()}>
            <div className="video-head">
              <strong>{activeVideo.title}</strong>
              <button className="ghost" onClick={() => setActiveVideo(null)}>关闭</button>
            </div>
            <video ref={videoRef} controls />
          </div>
        </div>
      )}
    </div>
  );
}
