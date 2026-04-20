import { ChevronLeft, ChevronRight, MonitorSmartphone, Search, Smartphone } from "lucide-react";
import { useEffect, useRef, useState } from "react";

type HeaderProps = {
  searchKeyword: string;
  onSearchKeywordChange: (value: string) => void;
  onSearchSubmit: (value?: string) => void;
  searchHistory: string[];
  onPickHistory: (value: string) => void;
  onClearHistory: () => void;
};

export function Header({
  searchKeyword,
  onSearchKeywordChange,
  onSearchSubmit,
  searchHistory,
  onPickHistory,
  onClearHistory
}: HeaderProps) {
  const [open, setOpen] = useState(false);
  const shellRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (!shellRef.current) return;
      if (!shellRef.current.contains(event.target as Node)) {
        setOpen(false);
      }
    }

    window.addEventListener("mousedown", handleClickOutside);
    return () => window.removeEventListener("mousedown", handleClickOutside);
  }, []);

  return (
    <header className="header-bar">
      <div className="header-nav">
        <button className="icon-button subtle" type="button" aria-label="返回">
          <ChevronLeft size={18} />
        </button>
        <button className="icon-button subtle" type="button" aria-label="前进">
          <ChevronRight size={18} />
        </button>
      </div>

      <div className="header-center">
        <div className="search-shell" ref={shellRef}>
          <div className="search-input">
            <Search size={16} />
            <input
              value={searchKeyword}
              onChange={(event) => onSearchKeywordChange(event.target.value)}
              onFocus={() => setOpen(true)}
              onKeyDown={(event) => {
                if (event.key === "Enter") {
                  setOpen(false);
                  onSearchSubmit();
                }
              }}
              placeholder="搜索音乐 / 歌手"
            />
            <button
              type="button"
              className="text-button"
              onClick={() => {
                setOpen(false);
                onSearchSubmit();
              }}
            >
              搜索
            </button>
          </div>

          {open && (searchHistory.length > 0 || searchKeyword.trim().length > 0) && (
            <div className="search-dropdown">
              <div className="search-dropdown-head">
                <span>搜索建议</span>
                {searchHistory.length > 0 && (
                  <button type="button" className="text-button subtle" onClick={onClearHistory}>
                    清空历史
                  </button>
                )}
              </div>
              {searchKeyword.trim().length > 0 && (
                <button type="button" className="search-history-item" onClick={() => onSearchSubmit()}>
                  搜索 “{searchKeyword.trim()}”
                </button>
              )}
              {searchHistory.length > 0 ? (
                <div className="search-history-list">
                  {searchHistory.map((item) => (
                    <button
                      key={item}
                      type="button"
                      className="search-history-item"
                      onClick={() => {
                        setOpen(false);
                        onPickHistory(item);
                      }}
                    >
                      {item}
                    </button>
                  ))}
                </div>
              ) : (
                <p className="search-empty">输入关键词后回车，搜索记录会保存在本地浏览器。</p>
              )}
            </div>
          )}
        </div>

        <div className="header-promo">
          <div className="promo-badge">Web</div>
          <span>CloudMusic 在线音视频平台</span>
        </div>
      </div>

      <div className="header-actions">
        <button className="icon-button subtle" type="button" aria-label="移动设备">
          <Smartphone size={17} />
        </button>
        <button className="icon-button subtle" type="button" aria-label="多端协同">
          <MonitorSmartphone size={17} />
        </button>
      </div>
    </header>
  );
}
