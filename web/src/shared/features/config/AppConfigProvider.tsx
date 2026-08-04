import { createContext, useContext, type PropsWithChildren } from "react";

type AppConfig = {
  apiBaseUrl: string;
  streamPath: string;
};

type AppConfigContextValue = {
  config: AppConfig;
};

const AppConfigContext = createContext<AppConfigContextValue | null>(null);

function trimTrailingSlash(value: string) {
  return value.replace(/\/+$/, "");
}

const apiOrigin = trimTrailingSlash(import.meta.env.VITE_API_ORIGIN || "");
const config: AppConfig = {
  apiBaseUrl:
    import.meta.env.VITE_API_BASE_URL ||
    (apiOrigin ? `${apiOrigin}/master/api/v1` : "/master/api/v1"),
  streamPath:
    import.meta.env.VITE_STREAM_PATH ||
    (apiOrigin
      ? `${apiOrigin}/master/api/v1/stream/hosts`
      : "/master/api/v1/stream/hosts"),
};

export function AppConfigProvider({ children }: PropsWithChildren) {
  return <AppConfigContext.Provider value={{ config }}>{children}</AppConfigContext.Provider>;
}

export function useAppConfig() {
  const context = useContext(AppConfigContext);
  if (!context) {
    throw new Error("useAppConfig must be used within AppConfigProvider");
  }
  return context;
}
