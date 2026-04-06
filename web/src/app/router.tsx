import { createBrowserRouter } from "react-router-dom";
import { Shell } from "../components/layout/Shell";
import { DashboardPage } from "../pages/dashboard/DashboardPage";

export const router = createBrowserRouter([
  {
    path: "/",
    element: <Shell />,
    children: [
      {
        index: true,
        element: <DashboardPage />,
      },
    ],
  },
]);
