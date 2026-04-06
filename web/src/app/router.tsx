import { Navigate, createBrowserRouter, useParams } from "react-router-dom";
import { Shell } from "../components/layout/Shell";
import { DashboardPage } from "../pages/dashboard/DashboardPage";
import { MobileAgentPage } from "../pages/mobile/MobileAgentPage";

function MobilePwaRedirect() {
  const { tenantCode = "default" } = useParams();
  return <Navigate to={`/${tenantCode}/pwa`} replace />;
}

export const router = createBrowserRouter([
  {
    path: "/",
    element: <Navigate to="/default" replace />,
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
    element: <Shell />,
    children: [
      {
        index: true,
        element: <DashboardPage />,
      },
    ],
  },
]);
