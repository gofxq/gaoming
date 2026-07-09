import { useMemo } from "react";
import { Navigate, useLocation } from "react-router-dom";
import { useAuth } from "../../app/providers/AuthProvider";
import { useTenant } from "../../app/providers/TenantProvider";

export function LoginPage() {
  const location = useLocation();
  const { tenantCode } = useTenant();
  const { authenticated, initializing, user } = useAuth();
  const returnTo = useMemo(() => {
    const next = new URLSearchParams(location.search).get("return_to") || `/${tenantCode}`;
    return next.startsWith("/") ? next : `/${tenantCode}`;
  }, [location.search, tenantCode]);

  if (!initializing && authenticated && user) {
    return <Navigate to={returnTo || `/${user.tenant_code}`} replace />;
  }

  return (
    <div className="auth-page">
      <section className="auth-card">
        <div className="eyebrow">Gaoming Auth</div>
        <h1>登录暂未开放</h1>
        <p className="auth-copy">
          当前版本未配置网页登录方式。请联系管理员开通账号入口，或使用已有会话继续访问。
        </p>
        <div className="auth-meta">
          <span className="meta-pill">Tenant: {tenantCode}</span>
          <span className="meta-pill">{initializing ? "Checking Session" : "No Login Provider"}</span>
        </div>
        {!initializing ? <div className="auth-error">没有可用的网页登录方式。</div> : null}
      </section>
    </div>
  );
}
