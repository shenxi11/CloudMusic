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
import { Header } from "./components/layout/Header";
import { LyricsPanel } from "./components/layout/LyricsPanel";
import { PlayerBar } from "./components/layout/PlayerBar";
import { Sidebar } from "./components/layout/Sidebar";
import { VideoModal } from "./components/layout/VideoModal";
import type { AppTab, PlaybackMode, QueueItem, RecommendScene, TrackRowItem, UserSession } from "./models";
import { ConnectView } from "./views/ConnectView";
import { AuthView } from "./views/AuthView";
import { RecommendView } from "./views/RecommendView";
import { TrackLibraryView } from "./views/TrackLibraryView";
import { VideoView } from "./views/VideoView";
import { findActiveLyricIndex, formatTime, guessTitle, parseLyricText } from "./utils";

const STORAGE_SERVER = "cloudmusic_web_server";
const STORAGE_USER = "cloudmusic_web_user";
const STORAGE_SEARCH_HISTORY = "cloudmusic_web_search_history";

function durationLabel(duration?: string, durationSec?: number): string {
  if (duration && duration.trim().length > 0) return duration;
  if (!durationSec || !Number.isFinite(durationSec)) return "-";
  return formatTime(durationSec);
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
  const [searchHistory, setSearchHistory] = useState<string[]>(() => {
    const raw = localStorage.getItem(STORAGE_SEARCH_HISTORY);
    if (!raw) return [];
    try {
      const parsed = JSON.parse(raw);
      return Array.isArray(parsed) ? parsed.map((entry) => String(entry)) : [];
    } catch {
      return [];
    }
  });
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
  const [lyricLines, setLyricLines] = useState<ReturnType<typeof parseLyricText>>([]);
  const [activeLyricIndex, setActiveLyricIndex] = useState(-1);

  const [activeVideo, setActiveVideo] = useState<{ title: string; url: string } | null>(null);

  const audioRef = useRef<HTMLAudioElement | null>(null);
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const lyricBodyRef = useRef<HTMLDivElement | null>(null);

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
    const element = lyricBodyRef.current.querySelector<HTMLElement>(`[data-lyric-index="${activeLyricIndex}"]`);
    if (!element) return;
    element.scrollIntoView({ block: "center", behavior: "smooth" });
  }, [activeLyricIndex, lyricsOpen]);

  useEffect(() => {
    if (!isMobile) {
      setMobilePlayerOpen(false);
    }
  }, [isMobile]);

  useEffect(() => {
    const videoElement = videoRef.current;
    if (!videoElement || !activeVideo) return;

    let cancelled = false;
    let cleanup = () => {};

    const boot = async () => {
      const source = activeVideo.url;
      if (source.toLowerCase().includes(".m3u8")) {
        const module = await import("hls.js");
        if (cancelled) return;
        const Hls = module.default;
        if (Hls.isSupported()) {
          const hls = new Hls({ enableWorker: true, lowLatencyMode: true });
          hls.loadSource(source);
          hls.attachMedia(videoElement);
          cleanup = () => hls.destroy();
        } else {
          videoElement.src = source;
        }
      } else {
        videoElement.src = source;
      }
      void videoElement.play().catch(() => undefined);
    };

    void boot();
    return () => {
      cancelled = true;
      cleanup();
      videoElement.pause();
      videoElement.removeAttribute("src");
      videoElement.load();
    };
  }, [activeVideo]);

  const pageMeta = useMemo(() => {
    switch (tab) {
      case "music":
        return { title: "音乐库", subtitle: `当前已收录 ${musicList.length} 首歌曲` };
      case "video":
        return { title: "视频库", subtitle: `当前共有 ${videoList.length} 个视频资源` };
      case "favorites":
        return { title: "我的喜欢", subtitle: `你已收藏 ${favoriteList.length} 首歌曲` };
      case "search":
        return { title: "搜索结果", subtitle: searchKeyword ? `关键词：${searchKeyword}` : "使用顶部搜索框快速定位歌曲" };
      default:
        return { title: "今日推荐", subtitle: "按设计稿重做的浅色桌面风格推荐页面，继续使用真实推荐接口。" };
    }
  }, [favoriteList.length, musicList.length, searchKeyword, tab, videoList.length]);

  const musicRows = useMemo<TrackRowItem[]>(
    () =>
      musicList.map((item, index) => ({
        id: `${item.path}_${index}`,
        path: item.path,
        title: guessTitle(item.path),
        artist: item.artist || "未知歌手",
        album: "本地/在线曲库",
        durationLabel: durationLabel(item.duration),
        coverArtUrl: item.cover_art_url,
        meta: "音乐库"
      })),
    [musicList]
  );

  const searchRows = useMemo<TrackRowItem[]>(
    () =>
      searchList.map((item, index) => ({
        id: `${item.path}_${index}`,
        path: item.path,
        title: guessTitle(item.path),
        artist: item.artist || "未知歌手",
        album: "搜索结果",
        durationLabel: durationLabel(item.duration),
        coverArtUrl: item.cover_art_url,
        meta: "搜索命中"
      })),
    [searchList]
  );

  const favoriteRows = useMemo<TrackRowItem[]>(
    () =>
      favoriteList.map((item, index) => ({
        id: `${item.path}_${index}`,
        path: item.path,
        title: item.title || guessTitle(item.path),
        artist: item.artist || "未知歌手",
        album: item.album || "我的喜欢",
        durationLabel: durationLabel(item.duration),
        coverArtUrl: item.cover_art_url,
        meta: "已收藏"
      })),
    [favoriteList]
  );

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
    } catch (error) {
      if (!silent) {
        setConnectErr((error as Error).message || "连接失败");
      }
      setServerUrl("");
    } finally {
      setConnecting(false);
    }
  }

  async function onAuthSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
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
        if (!username.trim()) {
          throw new Error("注册模式下用户名不能为空");
        }
        await registerUser(serverUrl, account.trim(), password, username.trim());
      }

      const result = await login(serverUrl, account.trim(), password);
      if (!result.success_bool) {
        throw new Error("账号或密码错误");
      }

      const nextSession: UserSession = {
        account: account.trim(),
        username: result.username || username || account.trim()
      };
      setSession(nextSession);
      localStorage.setItem(STORAGE_USER, JSON.stringify(nextSession));
      setPassword("");
    } catch (error) {
      setAuthErr((error as Error).message || "认证失败");
    } finally {
      setAuthLoading(false);
    }
  }

  async function refreshData(baseUrl: string, currentSession: UserSession, scene: RecommendScene) {
    setLoadingData(true);
    setDataErr("");
    try {
      const [recommendRet, musicRet, videoRet, favoriteRet] = await Promise.allSettled([
        listRecommendations(baseUrl, currentSession.account, scene),
        listMusic(baseUrl),
        listVideos(baseUrl),
        listFavorites(baseUrl, currentSession.account)
      ]);

      if (recommendRet.status === "fulfilled") setRecommendData(recommendRet.value);
      if (musicRet.status === "fulfilled") setMusicList(musicRet.value);
      if (videoRet.status === "fulfilled") setVideoList(videoRet.value);
      if (favoriteRet.status === "fulfilled") setFavoriteList(favoriteRet.value);

      const errors = [recommendRet, musicRet, videoRet, favoriteRet]
        .filter((result) => result.status === "rejected")
        .map((result) => (result as PromiseRejectedResult).reason?.message || "数据加载失败");
      if (errors.length > 0) {
        setDataErr(errors.join(" | "));
      }
    } finally {
      setLoadingData(false);
    }
  }

  function updateSearchHistory(keyword: string) {
    const normalized = keyword.trim();
    if (!normalized) return;
    setSearchHistory((previous) => {
      const next = [normalized, ...previous.filter((item) => item !== normalized)].slice(0, 8);
      localStorage.setItem(STORAGE_SEARCH_HISTORY, JSON.stringify(next));
      return next;
    });
  }

  async function searchMusicList(keywordOverride?: string) {
    const keyword = (keywordOverride ?? searchKeyword).trim();
    if (!serverUrl || !keyword) return;
    setSearchLoading(true);
    setDataErr("");
    try {
      const result = await searchMusic(serverUrl, keyword);
      setSearchKeyword(keyword);
      setSearchList(result);
      setTab("search");
      updateSearchHistory(keyword);
    } catch (error) {
      setDataErr((error as Error).message || "搜索失败");
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

    const previous = currentTrack;
    if (emitSkip && previous?.recContext) {
      void postRecommendationFeedback(serverUrl, {
        user_id: session.account,
        song_id: previous.path,
        event_type: "skip",
        request_id: previous.recContext.requestId,
        model_version: previous.recContext.modelVersion,
        scene: previous.recContext.scene
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
          coverArtUrl: detail.album_cover_url || candidate.coverArtUrl,
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
    } catch (error) {
      setPlayErr((error as Error).message || "播放失败");
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
      artist: item.artist || "未知歌手",
      coverArtUrl: item.cover_art_url
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

  async function onLikeCurrent() {
    if (!session || !serverUrl || !currentTrack) return;
    try {
      await addFavorite(serverUrl, session.account, {
        music_path: currentTrack.path,
        music_title: currentTrack.title,
        artist: currentTrack.artist,
        duration_sec: currentTrack.durationSec,
        is_local: false
      });
      const favorites = await listFavorites(serverUrl, session.account);
      setFavoriteList(favorites);

      if (currentTrack.recContext) {
        await postRecommendationFeedback(serverUrl, {
          user_id: session.account,
          song_id: currentTrack.path,
          event_type: "like",
          request_id: currentTrack.recContext.requestId,
          model_version: currentTrack.recContext.modelVersion,
          scene: currentTrack.recContext.scene
        }).catch(() => undefined);
      }
    } catch (error) {
      setPlayErr((error as Error).message || "收藏失败");
    }
  }

  async function loadLyrics(url: string) {
    setLyricsLoading(true);
    setActiveLyricIndex(-1);
    try {
      const response = await fetch(url);
      const text = await response.text();
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
      const result = await getVideoStream(serverUrl, path);
      setActiveVideo({ title, url: result.url });
    } catch (error) {
      setDataErr((error as Error).message || "加载视频失败");
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
    const previous = queueIndex - 1;
    if (previous < 0) return;
    void playQueueIndex(queueItems, previous, true);
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
    setPlaybackMode((previous) => {
      if (previous === "seq") return "single";
      if (previous === "single") return "random";
      return "seq";
    });
  }

  function togglePlay() {
    const element = audioRef.current;
    if (!element) return;
    if (playing) {
      element.pause();
    } else {
      void element.play().catch(() => undefined);
    }
  }

  function seekAudio(value: number) {
    const element = audioRef.current;
    if (!element || !Number.isFinite(audioDurationSec) || audioDurationSec <= 0) return;
    const next = Math.max(0, Math.min(value, audioDurationSec));
    element.currentTime = next;
    setAudioCurrentSec(next);
  }

  function handleAudioLoadedMeta(event: SyntheticEvent<HTMLAudioElement>) {
    const element = event.currentTarget;
    setAudioDurationSec(Number.isFinite(element.duration) ? element.duration : 0);
  }

  function handleAudioTimeUpdate(event: SyntheticEvent<HTMLAudioElement>) {
    const element = event.currentTarget;
    setAudioCurrentSec(element.currentTime);
    if (lyricLines.length > 0) {
      setActiveLyricIndex(findActiveLyricIndex(lyricLines, element.currentTime));
    }
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
    setTab("recommend");
    localStorage.removeItem(STORAGE_USER);
  }

  if (!serverUrl) {
    return (
      <ConnectView
        serverInput={serverInput}
        connecting={connecting}
        error={connectErr}
        onChange={setServerInput}
        onSubmit={() => void tryConnect(serverInput)}
      />
    );
  }

  if (!session) {
    return (
      <AuthView
        authMode={authMode}
        serverUrl={serverUrl}
        account={account}
        password={password}
        username={username}
        loading={authLoading}
        error={authErr}
        onAccountChange={setAccount}
        onPasswordChange={setPassword}
        onUsernameChange={setUsername}
        onSubmit={onAuthSubmit}
        onSwitchMode={() => {
          setAuthMode(authMode === "login" ? "register" : "login");
          setAuthErr("");
        }}
        onChangeServer={() => setServerUrl("")}
      />
    );
  }

  return (
    <div className={`app-shell ${isMobile ? "is-mobile" : ""}`}>
      <Header
        searchKeyword={searchKeyword}
        onSearchKeywordChange={setSearchKeyword}
        onSearchSubmit={(value) => void searchMusicList(value)}
        searchHistory={searchHistory}
        onPickHistory={(value) => void searchMusicList(value)}
        onClearHistory={() => {
          setSearchHistory([]);
          localStorage.removeItem(STORAGE_SEARCH_HISTORY);
        }}
      />

      <div className="app-body">
        <Sidebar
          activeTab={tab}
          session={session}
          serverUrl={serverUrl}
          musicCount={musicList.length}
          favoriteCount={favoriteList.length}
          videoCount={videoList.length}
          loading={loadingData}
          onTabChange={setTab}
          onRefresh={() => void refreshData(serverUrl, session, recommendScene)}
          onLogout={logout}
        />

        <main className="page-shell">
          <div className="page-topbar">
            <div>
              <h2>{pageMeta.title}</h2>
              <p>{pageMeta.subtitle}</p>
            </div>
            {dataErr && <div className="inline-error">{dataErr}</div>}
          </div>

          {tab === "recommend" && (
            <RecommendView
              data={recommendData}
              scene={recommendScene}
              loading={loadingData && !recommendData}
              onSceneChange={setRecommendScene}
              onPlay={(index) => void playRecommendationAt(index)}
            />
          )}

          {tab === "music" && (
            <TrackLibraryView
              rows={musicRows}
              loading={loadingData && musicRows.length === 0}
              emptyTitle="音乐库为空"
              emptyDescription="当前服务端没有返回可播放的歌曲。"
              currentTrackPath={currentTrack?.path}
              isPlaying={playing}
              onPlayRow={(index) => void playMusicAt(musicList, index)}
              onToggleCurrent={togglePlay}
            />
          )}

          {tab === "search" && (
            <TrackLibraryView
              rows={searchRows}
              loading={searchLoading}
              emptyTitle="没有找到匹配歌曲"
              emptyDescription="换一个关键词再试，或者检查服务端搜索结果。"
              currentTrackPath={currentTrack?.path}
              isPlaying={playing}
              onPlayRow={(index) => void playMusicAt(searchList, index)}
              onToggleCurrent={togglePlay}
            />
          )}

          {tab === "favorites" && (
            <TrackLibraryView
              rows={favoriteRows}
              loading={loadingData && favoriteRows.length === 0}
              emptyTitle="还没有收藏歌曲"
              emptyDescription="在播放器里点击喜欢后，这里会展示你的收藏列表。"
              currentTrackPath={currentTrack?.path}
              isPlaying={playing}
              onPlayRow={(index) => void playFavoriteAt(favoriteList, index)}
              onToggleCurrent={togglePlay}
            />
          )}

          {tab === "video" && (
            <VideoView
              items={videoList}
              loading={loadingData && videoList.length === 0}
              onPlay={(path, title) => void playVideo(path, title)}
            />
          )}
        </main>
      </div>

      <audio
        ref={audioRef}
        className="hidden-audio"
        autoPlay
        src={currentTrack?.streamUrl}
        onPlay={() => setPlaying(true)}
        onPause={() => setPlaying(false)}
        onLoadedMetadata={handleAudioLoadedMeta}
        onDurationChange={handleAudioLoadedMeta}
        onTimeUpdate={handleAudioTimeUpdate}
        onEnded={handleAudioEnded}
      />

      <PlayerBar
        currentTrack={currentTrack}
        queueIndex={queueIndex}
        queueSize={queueItems.length}
        playing={playing}
        playbackMode={playbackMode}
        currentSec={audioCurrentSec}
        durationSec={audioDurationSec}
        lyricsOpen={lyricsOpen}
        mobileSheetOpen={mobilePlayerOpen}
        onPrev={playPrev}
        onNext={playNext}
        onTogglePlay={togglePlay}
        onCycleMode={cyclePlaybackMode}
        onToggleLyrics={() => setLyricsOpen((previous) => !previous)}
        onSeek={seekAudio}
        onLikeCurrent={() => void onLikeCurrent()}
        onOpenMobileSheet={() => setMobilePlayerOpen(true)}
        onCloseMobileSheet={() => setMobilePlayerOpen(false)}
      />

      {playErr && <div className="floating-error">{playErr}</div>}

      {lyricsOpen && (
        <LyricsPanel
          title={currentTrack?.title || "歌词"}
          loading={lyricsLoading}
          lines={lyricLines}
          activeIndex={activeLyricIndex}
          bodyRef={lyricBodyRef}
          onClose={() => setLyricsOpen(false)}
        />
      )}

      {activeVideo && (
        <VideoModal
          title={activeVideo.title}
          videoRef={videoRef}
          onClose={() => setActiveVideo(null)}
        />
      )}
    </div>
  );
}
