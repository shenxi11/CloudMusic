import { Music4 } from "lucide-react";

type ConnectViewProps = {
  serverInput: string;
  connecting: boolean;
  error: string;
  onChange: (value: string) => void;
  onSubmit: () => void;
};

export function ConnectView({ serverInput, connecting, error, onChange, onSubmit }: ConnectViewProps) {
  return (
    <div className="entry-shell">
      <div className="entry-card wide">
        <div className="entry-brand">
          <Music4 size={26} />
          <span>CloudMusic Web</span>
        </div>
        <div className="entry-copy">
          <h1>连接音乐服务端</h1>
          <p>输入服务端地址后进入完整的 Web 音视频界面，推荐、音乐库、视频与收藏都会从该地址读取。</p>
        </div>
        <div className="entry-form">
          <input value={serverInput} onChange={(event) => onChange(event.target.value)} placeholder="http://127.0.0.1:8080" />
          <button type="button" onClick={onSubmit} disabled={connecting}>
            {connecting ? "连接中..." : "连接服务器"}
          </button>
        </div>
        {error && <div className="form-error">{error}</div>}
      </div>
    </div>
  );
}
