import { Navigate, Outlet, createBrowserRouter, useLocation, useParams } from "react-router-dom";
import { useAuth } from "./providers/AuthProvider";
import { Shell } from "../components/layout/Shell";
import { LoginPage } from "../pages/auth/LoginPage";
import { UsersPage } from "../pages/admin/UsersPage";
import { PcDashboardPage } from "../pc/PcDashboardPage";

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
  return <Shell />;
}

export const router = createBrowserRouter([
  {
    path: "/",
    element: <RootRedirect />,
  },
  {
    path: "/:tenantCode",
    children: [
      {
        element: <TenantEntry />,
        children: [
          {
            index: true,
            element: <PcDashboardPage />,
          },
          {
            path: "users",
            element: <RequireAuth />,
            children: [
              {
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
