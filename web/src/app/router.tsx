import { Navigate, Outlet, createBrowserRouter, useLocation, useParams } from "react-router-dom";
import { useAuth } from "../shared/features/auth/AuthProvider";
import { PcShell } from "../pc/pages/PcShell";
import { LoginPage } from "../pc/pages/LoginPage";
import { SetupPage } from "../pc/pages/SetupPage";
import { UsersPage } from "../pc/pages/UsersPage";
import { PcDashboardPage } from "../pc/pages/PcDashboardPage";
import { SharePage } from "../pc/pages/SharePage";
import { SharesManagePage } from "../pc/pages/SharesManagePage";

function RootRedirect() {
  return <Navigate to="/default" replace />;
}

function RequireAuth() {
  const location = useLocation();
  const { tenantCode = "default" } = useParams();
  const { authenticated, initializing, user } = useAuth();

  if (initializing) {
    return <div className="auth-loading">正在检查登录状态...</div>;
  }
  if (!authenticated || !user) {
    const search = new URLSearchParams();
    search.set("return_to", `${location.pathname}${location.search}${location.hash}`);
    return <Navigate to={`/${tenantCode}/login?${search.toString()}`} replace />;
  }
  if (user.tenant_code !== tenantCode) {
    return <div className="auth-page"><section className="auth-card"><h1>无权查看此空间</h1><p>当前账号只能访问自己的空间，请返回已授权的空间。</p><a href={`/${encodeURIComponent(user.tenant_code)}`}>返回自己的空间</a></section></div>;
  }
  return <Outlet />;
}

function RequireAdmin() {
  const { user } = useAuth();
  if (!user || user.role !== "admin") {
    return <div className="auth-card" role="alert"><h1>需要管理员权限</h1><p>当前账号可以查看主机，但不能管理成员或分享。</p></div>;
  }
  return <Outlet />;
}

function TenantEntry() {
  return <PcShell />;
}

export const router = createBrowserRouter([
  {
    path: "/",
    element: <RootRedirect />,
  },
  {
    path: "/:tenantCode",
    element: <RequireAuth />,
    children: [
      {
        element: <TenantEntry />,
        children: [
          { index: true, element: <PcDashboardPage /> },
          {
            element: <RequireAdmin />,
            children: [
              { path: "users", element: <UsersPage /> },
              { path: "shares", element: <SharesManagePage /> },
            ],
          },
        ],
      },
    ],
  },
  {
    path: "/:tenantCode/login",
    element: <LoginPage />,
  },
  { path: "/:tenantCode/setup", element: <SetupPage /> },
  { path: "/share", element: <SharePage /> },
  { path: "/demo", element: <SharePage /> },
  {
    path: "*",
    element: <Navigate to="/default" replace />,
  },
]);
