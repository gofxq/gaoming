import { Link, Outlet, useNavigate } from "react-router-dom";
import { useAuth } from "../../app/providers/AuthProvider";
import { useTenant } from "../../app/providers/TenantProvider";

export function Shell() {
  const { tenantCode } = useTenant();
  const { user, signOut } = useAuth();
  const navigate = useNavigate();

  async function handleSignOut() {
    await signOut();
    navigate(`/${tenantCode}/login`, { replace: true });
  }

  return (
    <div className="shell">
      <header className="topbar">
        <div>
          <div className="eyebrow">Gaoming Web</div>
          <h1 className="headline">实时监控看板</h1>
          <p className="headline-copy">
            单页面 React 应用，优先承接核心指标、窗口趋势和 H5 展示。
          </p>
        </div>
        <div className="shell-actions">
          <nav className="shell-badges">
            <Link to={`/${tenantCode}`} className="meta-pill">
              Dashboard
            </Link>
            {user?.role === "admin" ? (
              <Link to={`/${tenantCode}/users`} className="meta-pill">
                Users
              </Link>
            ) : null}
            <Link to={`/${tenantCode}/pwa`} className="meta-pill">
              Tenant PWA
            </Link>
            <span className="meta-pill">Tenant: {tenantCode}</span>
          </nav>
          <div className="shell-user">
            <span className="meta-pill">{user?.display_name || "未登录"}</span>
            {user ? (
              <>
                <span className="meta-pill">{user.role === "admin" ? "管理员" : "成员"}</span>
                <button type="button" className="meta-pill meta-pill-button" onClick={() => void handleSignOut()}>
                  退出登录
                </button>
              </>
            ) : null}
          </div>
        </div>
      </header>

      <main className="content">
        <Outlet />
      </main>
    </div>
  );
}
