export type ApiEnvelope<T> = {
  code: number;
  message: string;
  data: T;
};

export type PingData = {
  service: string;
  status: string;
  api_version: string;
  timestamp: number;
  server_time: string;
};

export type LoginResponse = {
  success: "true" | "false";
  success_bool: boolean;
  username: string;
  song_path_list: string[];
  online_session_token?: string;
};

export type MusicFileItem = {
  path: string;
  duration?: string;
  artist?: string;
  cover_art_url?: string;
};

export type MusicDetail = {
  stream_url: string;
  lrc_url?: string;
  album_cover_url?: string;
  duration?: number;
  title?: string;
  artist?: string;
  album?: string;
};

export type VideoFile = {
  name: string;
  path: string;
  size: number;
};

export type RecommendationItem = {
  song_id: string;
  path: string;
  title: string;
  artist: string;
  album?: string;
  duration_sec?: number;
  cover_art_url?: string;
  stream_url: string;
  lrc_url?: string;
  score: number;
  reason: string;
  source: "cf" | "content" | "hot" | "hybrid";
};

export type RecommendationData = {
  request_id: string;
  user_id: string;
  scene: string;
  model_version: string;
  items: RecommendationItem[];
};

export type FavoriteItem = {
  path: string;
  title?: string;
  artist?: string;
  album?: string;
  duration?: string;
  cover_art_url?: string;
};

function trimBase(baseUrl: string): string {
  return baseUrl.replace(/\/+$/, "");
}

async function request<T>(baseUrl: string, path: string, init?: RequestInit): Promise<T> {
  const isBodyMethod = (init?.method || "GET").toUpperCase() !== "GET";
  const resp = await fetch(`${trimBase(baseUrl)}${path}`, {
    headers: {
      ...(isBodyMethod ? { "Content-Type": "application/json" } : {}),
      ...(init?.headers || {})
    },
    ...init
  });
  const text = await resp.text();
  let data: unknown = null;
  try {
    data = text ? JSON.parse(text) : null;
  } catch {
    data = text;
  }
  if (!resp.ok) {
    const msg =
      typeof data === "object" && data !== null && "message" in data
        ? String((data as { message: string }).message)
        : text || `HTTP ${resp.status}`;
    throw new Error(msg);
  }
  return data as T;
}

export async function pingServer(baseUrl: string): Promise<PingData> {
  const ret = await request<ApiEnvelope<PingData>>(baseUrl, "/client/ping");
  return ret.data;
}

export async function login(baseUrl: string, account: string, password: string): Promise<LoginResponse> {
  return request<LoginResponse>(baseUrl, "/users/login", {
    method: "POST",
    body: JSON.stringify({ account, password })
  });
}

export async function registerUser(baseUrl: string, account: string, password: string, username: string): Promise<void> {
  await request<ApiEnvelope<{ success: boolean }>>(baseUrl, "/users/register", {
    method: "POST",
    body: JSON.stringify({ account, password, username })
  });
}

export async function listMusic(baseUrl: string): Promise<MusicFileItem[]> {
  return request<MusicFileItem[]>(baseUrl, "/files");
}

export async function searchMusic(baseUrl: string, keyword: string): Promise<MusicFileItem[]> {
  return request<MusicFileItem[]>(baseUrl, "/music/search", {
    method: "POST",
    body: JSON.stringify({ keyword })
  });
}

export async function getMusicDetail(baseUrl: string, path: string): Promise<MusicDetail> {
  return request<MusicDetail>(baseUrl, `/get_music?path=${encodeURIComponent(path)}`);
}

export async function listVideos(baseUrl: string): Promise<VideoFile[]> {
  return request<VideoFile[]>(baseUrl, "/videos");
}

export async function getVideoStream(baseUrl: string, path: string): Promise<{ url: string }> {
  return request<{ url: string }>(baseUrl, "/video/stream", {
    method: "POST",
    body: JSON.stringify({ path })
  });
}

export async function listRecommendations(
  baseUrl: string,
  userId: string,
  scene: "home" | "radio" | "detail" = "home"
): Promise<RecommendationData> {
  const ret = await request<ApiEnvelope<RecommendationData>>(
    baseUrl,
    `/recommendations/audio?user_id=${encodeURIComponent(userId)}&limit=24&scene=${encodeURIComponent(scene)}&exclude_played=true`
  );
  return ret.data;
}

export async function postRecommendationFeedback(
  baseUrl: string,
  payload: {
    user_id: string;
    song_id: string;
    event_type: "play" | "finish" | "skip" | "like";
    request_id?: string;
    model_version?: string;
    play_ms?: number;
    duration_ms?: number;
    scene?: "home" | "radio" | "detail";
  }
): Promise<void> {
  await request<ApiEnvelope<{ success: boolean }>>(baseUrl, "/recommendations/feedback", {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export async function listFavorites(baseUrl: string, userAccount: string): Promise<FavoriteItem[]> {
  return request<FavoriteItem[]>(baseUrl, `/user/favorites?user_account=${encodeURIComponent(userAccount)}`, {
    headers: {
      "X-User-Account": userAccount
    }
  });
}

export async function addFavorite(
  baseUrl: string,
  userAccount: string,
  payload: {
    music_path: string;
    music_title?: string;
    artist?: string;
    duration_sec?: number;
    is_local?: boolean;
  }
): Promise<void> {
  await request<{ success: boolean }>(baseUrl, `/user/favorites/add?user_account=${encodeURIComponent(userAccount)}`, {
    method: "POST",
    headers: {
      "X-User-Account": userAccount
    },
    body: JSON.stringify(payload)
  });
}

export async function addPlayHistory(
  baseUrl: string,
  userAccount: string,
  payload: {
    music_path: string;
    music_title?: string;
    artist?: string;
    album?: string;
    duration_sec?: number;
    is_local?: boolean;
  }
): Promise<void> {
  await request<{ success: boolean }>(baseUrl, `/user/history/add?user_account=${encodeURIComponent(userAccount)}`, {
    method: "POST",
    headers: {
      "X-User-Account": userAccount
    },
    body: JSON.stringify(payload)
  });
}
