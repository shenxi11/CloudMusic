import { FormEvent } from "react";
import { UserRound } from "lucide-react";

type AuthViewProps = {
  authMode: "login" | "register";
  serverUrl: string;
  account: string;
  password: string;
  username: string;
  loading: boolean;
  error: string;
  onAccountChange: (value: string) => void;
  onPasswordChange: (value: string) => void;
  onUsernameChange: (value: string) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  onSwitchMode: () => void;
  onChangeServer: () => void;
};

export function AuthView({
  authMode,
  serverUrl,
  account,
  password,
  username,
  loading,
  error,
  onAccountChange,
  onPasswordChange,
  onUsernameChange,
  onSubmit,
  onSwitchMode,
  onChangeServer
}: AuthViewProps) {
  return (
    <div className="entry-shell">
      <form className="entry-card" onSubmit={onSubmit}>
        <div className="entry-brand">
          <UserRound size={24} />
          <span>{authMode === "login" ? "用户登录" : "用户注册"}</span>
        </div>
        <div className="entry-copy">
          <h1>{authMode === "login" ? "欢迎回来" : "创建账号"}</h1>
          <p>当前服务端：{serverUrl}</p>
        </div>
        <div className="stack-field">
          <input value={account} onChange={(event) => onAccountChange(event.target.value)} placeholder="账号" />
          <input value={password} type="password" onChange={(event) => onPasswordChange(event.target.value)} placeholder="密码" />
          {authMode === "register" && (
            <input value={username} onChange={(event) => onUsernameChange(event.target.value)} placeholder="用户名" />
          )}
        </div>
        <button type="submit" disabled={loading}>
          {loading ? "提交中..." : authMode === "login" ? "登录" : "注册并登录"}
        </button>
        <div className="entry-actions">
          <button type="button" className="text-button subtle" onClick={onSwitchMode}>
            {authMode === "login" ? "没有账号？去注册" : "已有账号？去登录"}
          </button>
          <button type="button" className="text-button subtle" onClick={onChangeServer}>
            切换服务器
          </button>
        </div>
        {error && <div className="form-error">{error}</div>}
      </form>
    </div>
  );
}
