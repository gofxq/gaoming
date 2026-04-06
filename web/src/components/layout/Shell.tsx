import { Link, Outlet } from "react-router-dom";
import { useTenant } from "../../app/providers/TenantProvider";

export function Shell() {
  const { tenantCode } = useTenant();

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
        <div className="shell-badges">
          <span className="meta-pill">SPA</span>
          <span className="meta-pill">H5 Ready</span>
          <Link to={`/${tenantCode}/pwa`} className="meta-pill">
            Tenant PWA
          </Link>
          <span className="meta-pill">Tenant: {tenantCode}</span>
        </div>
      </header>

      <main className="content">
        <Outlet />
      </main>
    </div>
  );
}
