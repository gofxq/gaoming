import { Navigate, Outlet, createBrowserRouter, useLocation, useParams } from "react-router-dom";
import { shouldUsePwaLayout } from "./device";
import { useAuth } from "./providers/AuthProvider";
import { Shell } from "../components/layout/Shell";
import { DashboardPage } from "../pages/dashboard/DashboardPage";
import { MobileAgentPage } from "../pages/mobile/MobileAgentPage";
import { LoginPage } from "../pages/auth/LoginPage";
import { UsersPage } from "../pages/admin/UsersPage";

function RootRedirect() {
  return <Navigate to={shouldUsePwaLayout() ? "/default/pwa" : "/default"} replace />;
}

function MobilePwaRedirect() {
  const { tenantCode = "default" } = useParams();
  return <Navigate to={`/${tenantCode}/pwa`} replace />;
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
    return <Navigate to={location.pathname.replace(`/${tenantCode}`, `/${user.tenant_code}`)} replace />;
  }
  return <Outlet />;
}

function RequireAdmin() {
  const { user } = useAuth();
  if (!user || user.role !== "admin") {
    return <Navigate to=".." replace />;
  }
  return <Outlet />;
}

function TenantEntry() {
  const { tenantCode = "default" } = useParams();
  if (shouldUsePwaLayout()) {
    return <Navigate to={`/${tenantCode}/pwa`} replace />;
  }

  return <Shell />;
}

export const router = createBrowserRouter([
  {
    path: "/",
    element: <RootRedirect />,
  },
  {
    path: "/:tenantCode/mobile",
    element: <MobilePwaRedirect />,
  },
  {
    path: "/:tenantCode/pwa",
    element: <MobileAgentPage />,
  },
  {
    path: "/:tenantCode/pwa/:hostUID",
    element: <MobileAgentPage />,
  },
  {
    path: "/:tenantCode",
    element: <RequireAuth />,
    children: [
      {
        element: <TenantEntry />,
        children: [
          {
            index: true,
            element: <DashboardPage />,
          },
          {
            path: "users",
            element: <RequireAdmin />,
            children: [
              {
                index: true,
                element: <UsersPage />,
              },
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
  {
    path: "*",
    element: <Navigate to="/default" replace />,
  },
]);
