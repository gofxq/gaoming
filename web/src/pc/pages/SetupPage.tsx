import { useState, type FormEvent } from "react";
import { Link, Navigate } from "react-router-dom";
import { useAuth } from "../../shared/features/auth/AuthProvider";
import { useAppConfig } from "../../shared/features/config/AppConfigProvider";
import { useTenant } from "../../shared/features/tenant/TenantProvider";
import { HttpError, requestJson } from "../../shared/lib/http";
import { AppearanceControls } from "../components/appearance/AppearanceControls";

export function SetupPage() {
  const { tenantCode } = useTenant();
  const { config } = useAppConfig();
  const { authenticated, user, refreshSession } = useAuth();
  const [setupToken, setSetupToken] = useState("");
  const [username, setUsername] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function submit(event: FormEvent) {
    event.preventDefault(); if (busy) return;
    const bytes = new TextEncoder().encode(password).length;
    if (bytes < 12 || bytes > 72) { setError("密码长度须为 12～72 字节（中文等字符可能占多个字节）。"); return; }
    setBusy(true); setError("");
    try {
      await requestJson(`${config.apiBaseUrl}/auth/setup`, {
        method: "POST", credentials: "include", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ setup_token: setupToken, tenant_code: tenantCode, username, display_name: displayName, password }),
      });
      setPassword(""); setSetupToken("");
      if (!await refreshSession()) setError("管理员已创建，但无法读取会话。请检查 HTTPS/Cookie 配置，然后使用登录入口。");
    } catch (cause) {
      setError(cause instanceof HttpError && (cause.status === 403 || cause.status === 503) ? "初始化不可用或已经完成。请使用登录入口，或联系管理员。" : "初始化失败，请检查引导凭据、字段和网络后重试。");
    } finally { setBusy(false); }
  }

  if (authenticated && user) return <Navigate to={`/${user.tenant_code}`} replace />;

  return (
    <div className="auth-page">
      <AppearanceControls className="auth-appearance-controls" />
      <section className="auth-card">
        <div className="eyebrow">安全访问</div><h1>创建初始管理员</h1>
        <p className="auth-copy">空间 {tenantCode}。使用部署管理员提供的一次性引导凭据；该流程只在尚无用户的环境可用。</p>
        <form className="auth-form" onSubmit={(event) => void submit(event)}>
          <label>引导凭据<input type="password" value={setupToken} onChange={(event) => setSetupToken(event.target.value)} autoComplete="off" required disabled={busy} /></label>
          <label>用户名<input aria-label="用户名" aria-describedby="setup-username-help" value={username} onChange={(event) => setUsername(event.target.value)} pattern="[a-zA-Z0-9][a-zA-Z0-9_.-]{2,63}" autoComplete="username" required disabled={busy} /><small id="setup-username-help">3～64 位字母、数字、点、下划线或短横线，字母不区分大小写。</small></label>
          <label>显示名称<input value={displayName} onChange={(event) => setDisplayName(event.target.value)} maxLength={128} required disabled={busy} /></label>
          <label>密码<input aria-label="密码" aria-describedby="setup-password-help" type="password" value={password} onChange={(event) => setPassword(event.target.value)} autoComplete="new-password" required disabled={busy} /><small id="setup-password-help">12～72 字节</small></label>
          {error ? <div className="auth-error" role="alert">{error}</div> : null}
          <button type="submit" className="btn btn-primary" disabled={busy}>{busy ? "正在创建…" : "创建并登录"}</button>
        </form>
        <Link to={`/${tenantCode}/login`}>返回登录</Link>
      </section>
    </div>
  );
}
