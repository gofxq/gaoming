import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  type PropsWithChildren,
} from "react";
import { METRICS, type MetricKey } from "../../pages/dashboard/dashboard";

const APP_CONFIG_STORAGE_KEY = "gaoming:web:app-config:v2";

type AppConfigStorage = {
  visibleWindowMetrics?: MetricKey[];
};

type AppConfig = {
  apiBaseUrl: string;
  streamPath: string;
  visibleWindowMetrics: MetricKey[];
};

type AppConfigContextValue = {
  config: AppConfig;
  updateConfig: (patch: Partial<AppConfig>) => void;
};

const AppConfigContext = createContext<AppConfigContextValue | null>(null);

function trimTrailingSlash(value: string) {
  return value.replace(/\/+$/, "");
}

function defaultVisibleWindowMetrics(): MetricKey[] {
  return [
    "cpu_usage_pct",
    "mem_used_pct",
    "net_rx_bps",
    "net_tx_bps",
    "disk_read_bps",
    "disk_write_bps",
  ];
}

function sanitizeMetricKeys(keys: unknown): MetricKey[] {
  if (!Array.isArray(keys)) {
    return defaultVisibleWindowMetrics();
  }

  const allowed = new Set(METRICS.map((metric) => metric.key));
  const unique = new Set<MetricKey>();
  for (const key of keys) {
    if (typeof key !== "string" || !allowed.has(key as MetricKey)) {
      continue;
    }
    unique.add(key as MetricKey);
  }

  return unique.size ? [...unique] : defaultVisibleWindowMetrics();
}

function readStoredConfig(): AppConfigStorage | null {
  if (typeof window === "undefined") {
    return null;
  }

  try {
    const raw = window.localStorage.getItem(APP_CONFIG_STORAGE_KEY);
    return raw ? (JSON.parse(raw) as AppConfigStorage) : null;
  } catch {
    return null;
  }
}

const apiOrigin = trimTrailingSlash(import.meta.env.VITE_API_ORIGIN || "");
const storedConfig = readStoredConfig();

const defaultConfig: AppConfig = {
  apiBaseUrl:
    import.meta.env.VITE_API_BASE_URL ||
    (apiOrigin ? `${apiOrigin}/master/api/v1` : "/master/api/v1"),
  streamPath:
    import.meta.env.VITE_STREAM_PATH ||
    (apiOrigin
      ? `${apiOrigin}/master/api/v1/stream/hosts`
      : "/master/api/v1/stream/hosts"),
  visibleWindowMetrics: sanitizeMetricKeys(storedConfig?.visibleWindowMetrics),
};

export function AppConfigProvider({ children }: PropsWithChildren) {
  const [config, setConfig] = useState<AppConfig>(defaultConfig);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }

    const payload: AppConfigStorage = {
      visibleWindowMetrics: config.visibleWindowMetrics,
    };

    try {
      window.localStorage.setItem(APP_CONFIG_STORAGE_KEY, JSON.stringify(payload));
    } catch {
      // Ignore storage failures and keep in-memory config working.
    }
  }, [config.visibleWindowMetrics]);

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
