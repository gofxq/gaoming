import { useEffect, useMemo, useState, type FormEvent } from "react";
import { Link, Navigate, useLocation } from "react-router-dom";
import { useAuth } from "../../shared/features/auth/AuthProvider";
import { useAppConfig } from "../../shared/features/config/AppConfigProvider";
import { useTenant } from "../../shared/features/tenant/TenantProvider";
import { safeReturnTo } from "../../shared/features/auth/navigation";
import { HttpError, requestJson } from "../../shared/lib/http";
import { AppearanceControls } from "../components/appearance/AppearanceControls";

export function LoginPage() {
  const location = useLocation();
  const { tenantCode } = useTenant();
  const { config } = useAppConfig();
  const { authenticated, initializing, user, refreshSession } = useAuth();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [setupAvailable, setSetupAvailable] = useState(false);
  const returnTo = useMemo(() => safeReturnTo(new URLSearchParams(location.search).get("return_to"), user?.tenant_code || tenantCode), [location.search, tenantCode, user?.tenant_code]);

  useEffect(() => {
    const controller = new AbortController();
    void requestJson<{ available: boolean }>(`${config.apiBaseUrl}/auth/setup-status`, { signal: controller.signal, credentials: "include" })
      .then((result) => setSetupAvailable(result.available)).catch(() => {});
    return () => controller.abort();
  }, [config.apiBaseUrl]);

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (busy) return;
    setBusy(true); setError("");
    try {
      await requestJson(`${config.apiBaseUrl}/auth/login`, {
        method: "POST", credentials: "include", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ tenant_code: tenantCode, username, password }),
      });
      setPassword("");
      if (!await refreshSession()) setError("登录请求已完成，但无法读取会话。请检查 HTTPS/Cookie 配置后重试。");
    } catch (cause) {
      setError(cause instanceof HttpError && cause.status === 401 ? "空间、用户名或密码不正确，或账号已被停用。" : "登录失败，请检查网络或联系管理员后重试。");
    } finally { setBusy(false); }
  }

  if (!initializing && authenticated && user) return <Navigate to={returnTo} replace />;

  return (
    <div className="auth-page">
      <AppearanceControls className="auth-appearance-controls" />
      <section className="auth-card">
        <div className="eyebrow">安全访问</div>
        <h1>登录高明</h1>
        <p className="auth-copy">登录空间 {tenantCode}，查看已授权的主机与成员。</p>
        <form className="auth-form" onSubmit={(event) => void submit(event)}>
          <label>用户名<input value={username} onChange={(event) => setUsername(event.target.value)} autoComplete="username" required maxLength={64} disabled={busy} /></label>
          <label>密码<input type="password" value={password} onChange={(event) => setPassword(event.target.value)} autoComplete="current-password" required disabled={busy} /></label>
          {error ? <div className="auth-error" role="alert">{error}</div> : null}
          <button type="submit" className="btn btn-primary" disabled={busy || initializing}>{busy ? "正在登录…" : "登录"}</button>
        </form>
        {setupAvailable ? <Link to={`/${tenantCode}/setup`}>首次使用：创建初始管理员</Link> : <p className="auth-copy">没有账号或忘记密码？请联系空间管理员。管理员可使用本机管理命令恢复访问。</p>}
      </section>
    </div>
  );
}
