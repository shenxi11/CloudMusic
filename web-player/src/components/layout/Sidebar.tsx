import { Clapperboard, Heart, Home, LibraryBig, LogOut, RefreshCcw } from "lucide-react";
import type { AppTab, UserSession } from "../../models";

type SidebarProps = {
  activeTab: AppTab;
  session: UserSession;
  serverUrl: string;
  musicCount: number;
  favoriteCount: number;
  videoCount: number;
  loading: boolean;
  onTabChange: (tab: Exclude<AppTab, "search">) => void;
  onRefresh: () => void;
  onLogout: () => void;
};

const navItems = [
  { id: "recommend", label: "推荐", icon: Home },
  { id: "music", label: "音乐库", icon: LibraryBig },
  { id: "video", label: "视频库", icon: Clapperboard },
  { id: "favorites", label: "我的喜欢", icon: Heart }
] as const;

export function Sidebar({
  activeTab,
  session,
  serverUrl,
  musicCount,
  favoriteCount,
  videoCount,
  loading,
  onTabChange,
  onRefresh,
  onLogout
}: SidebarProps) {
  const stats: Record<string, string> = {
    music: `${musicCount} 首`,
    video: `${videoCount} 个`,
    favorites: `${favoriteCount} 首`
  };

  return (
    <aside className="sidebar">
      <div className="profile-card">
        <div className="profile-avatar">{session.username.slice(0, 1).toUpperCase()}</div>
        <div className="profile-meta">
          <strong>{session.username || session.account}</strong>
          <span>{session.account}</span>
          <small>{serverUrl}</small>
        </div>
      </div>

      <div className="sidebar-banner">
        <div>
          <strong>桌面风格 Web 端</strong>
          <p>推荐、音乐库、视频和喜欢均已接入真实服务端数据。</p>
        </div>
      </div>

      <nav className="sidebar-nav">
        {navItems.map(({ id, label, icon: Icon }) => (
          <button
            key={id}
            type="button"
            className={`sidebar-link ${activeTab === id ? "is-active" : ""}`}
            onClick={() => onTabChange(id)}
          >
            <span className="sidebar-link-main">
              <Icon size={18} />
              <span>{label}</span>
            </span>
            {stats[id] && <small>{stats[id]}</small>}
          </button>
        ))}
      </nav>

      <div className="sidebar-footer">
        <button type="button" className="sidebar-action" onClick={onRefresh} disabled={loading}>
          <RefreshCcw size={16} />
          <span>{loading ? "刷新中..." : "刷新数据"}</span>
        </button>
        <button type="button" className="sidebar-action" onClick={onLogout}>
          <LogOut size={16} />
          <span>退出登录</span>
        </button>
      </div>
    </aside>
  );
}
