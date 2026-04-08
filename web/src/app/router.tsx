import { Navigate, createBrowserRouter, useParams } from "react-router-dom";
import { shouldUsePwaLayout } from "./device";
import { Shell } from "../components/layout/Shell";
import { DashboardPage } from "../pages/dashboard/DashboardPage";
import { MobileAgentPage } from "../pages/mobile/MobileAgentPage";

function RootRedirect() {
  return <Navigate to={shouldUsePwaLayout() ? "/default/pwa" : "/default"} replace />;
}

function MobilePwaRedirect() {
  const { tenantCode = "default" } = useParams();
  return <Navigate to={`/${tenantCode}/pwa`} replace />;
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
    element: <TenantEntry />,
    children: [
      {
        index: true,
        element: <DashboardPage />,
      },
    ],
  },
]);
