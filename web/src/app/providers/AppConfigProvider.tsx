import {
  createContext,
  useContext,
  useMemo,
  useState,
  type PropsWithChildren,
} from "react";

type AppConfig = {
  apiBaseUrl: string;
  streamPath: string;
};

type AppConfigContextValue = {
  config: AppConfig;
  updateConfig: (patch: Partial<AppConfig>) => void;
};

const AppConfigContext = createContext<AppConfigContextValue | null>(null);

function trimTrailingSlash(value: string) {
  return value.replace(/\/+$/, "");
}

const apiOrigin = trimTrailingSlash(import.meta.env.VITE_API_ORIGIN || "");

const defaultConfig: AppConfig = {
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
  const [config, setConfig] = useState<AppConfig>(defaultConfig);

  const value = useMemo<AppConfigContextValue>(
    () => ({
      config,
      updateConfig: (patch) => {
        setConfig((current) => ({ ...current, ...patch }));
      },
    }),
    [config],
  );

  return <AppConfigContext.Provider value={value}>{children}</AppConfigContext.Provider>;
}

export function useAppConfig() {
  const context = useContext(AppConfigContext);
  if (!context) {
    throw new Error("useAppConfig must be used within AppConfigProvider");
  }
  return context;
}
