export type MetricKey =
  | "cpu_usage_pct"
  | "mem_used_pct"
  | "disk_used_pct"
  | "load1"
  | "net_rx_bps"
  | "net_tx_bps";

export type HostSnapshot = {
  host_uid: string;
  hostname?: string;
  primary_ip?: string;
  overall_state?: number;
  agent_state?: number;
  cpu_usage_pct?: number;
  mem_used_pct?: number;
  disk_used_pct?: number;
  disk_read_bps?: number;
  disk_write_bps?: number;
  load1?: number;
  net_rx_bps?: number;
  net_tx_bps?: number;
  last_agent_seen_at?: string;
  last_metric_at?: string;
  version?: number;
};

export type MetricPoint = {
  ts: string;
  value: number;
};

export type MetricLatestMap = Partial<Record<MetricKey, MetricPoint>>;
export type HostHistoryMap = Partial<Record<MetricKey, MetricPoint[]>>;

export type HostSyncPayload = {
  items?: HostSnapshot[];
  latest?: Record<string, MetricLatestMap>;
  server_time?: string;
};

export type HostUpsertPayload = {
  item?: HostSnapshot;
  latest?: MetricLatestMap;
  server_time?: string;
};

export type HostDeletePayload = {
  host_uid?: string;
  server_time?: string;
};

export type MetricDefinition = {
  key: MetricKey;
  label: string;
  unit: string;
  source: (item: HostSnapshot) => number;
};

export const WINDOW_OPTIONS = [
  { label: "1m", seconds: 60 },
  { label: "5m", seconds: 300 },
  { label: "15m", seconds: 900 },
  { label: "1h", seconds: 3600 },
];

export const METRICS: MetricDefinition[] = [
  {
    key: "cpu_usage_pct",
    label: "CPU",
    unit: "%",
    source: (item) => Number(item.cpu_usage_pct || 0),
  },
  {
    key: "mem_used_pct",
    label: "内存",
    unit: "%",
    source: (item) => Number(item.mem_used_pct || 0),
  },
  {
    key: "disk_used_pct",
    label: "磁盘用量",
    unit: "%",
    source: (item) => Number(item.disk_used_pct || 0),
  },
  {
    key: "load1",
    label: "负载",
    unit: "",
    source: (item) => Number(item.load1 || 0),
  },
  {
    key: "net_rx_bps",
    label: "网络 RX",
    unit: "B/s",
    source: (item) => Number(item.net_rx_bps || 0),
  },
  {
    key: "net_tx_bps",
    label: "网络 TX",
    unit: "B/s",
    source: (item) => Number(item.net_tx_bps || 0),
  },
];

export const STATE_LABELS: Record<number, string> = {
  0: "UNKNOWN",
  1: "UP",
  2: "WARNING",
  3: "CRITICAL",
  4: "OFFLINE",
  5: "MAINTENANCE",
  6: "DISABLED",
};

export const STATE_CLASSES: Record<number, string> = {
  0: "unknown",
  1: "up",
  2: "warning",
  3: "critical",
  4: "offline",
  5: "maintenance",
  6: "disabled",
};

export function sortHosts(items: HostSnapshot[]) {
  return [...items].sort((left, right) => {
    const leftOffline = left.overall_state === 4 ? 1 : 0;
    const rightOffline = right.overall_state === 4 ? 1 : 0;
    if (leftOffline !== rightOffline) {
      return leftOffline - rightOffline;
    }
    return left.host_uid.localeCompare(right.host_uid);
  });
}

export function listToHostMap(items: HostSnapshot[]) {
  const next: Record<string, HostSnapshot> = {};
  for (const item of items) {
    next[item.host_uid] = item;
  }
  return next;
}

export function formatPercent(value: number) {
  return `${Number(value || 0).toFixed(1)}%`;
}

export function formatBps(value: number) {
  const units = ["B/s", "KB/s", "MB/s", "GB/s"];
  let current = Number(value || 0);
  let idx = 0;

  while (current >= 1024 && idx < units.length - 1) {
    current /= 1024;
    idx += 1;
  }

  return `${current.toFixed(current >= 10 || idx === 0 ? 0 : 1)} ${units[idx]}`;
}

export function formatMetricValue(metricKey: MetricKey, value: number) {
  switch (metricKey) {
    case "cpu_usage_pct":
    case "mem_used_pct":
    case "disk_used_pct":
      return formatPercent(value);
    case "net_rx_bps":
    case "net_tx_bps":
      return formatBps(value);
    default:
      return Number(value || 0).toFixed(2);
  }
}

export function formatAgo(isoTime?: string) {
  if (!isoTime || isoTime.startsWith("0001-01-01")) {
    return "--";
  }

  const diff = Math.max(0, Math.floor((Date.now() - new Date(isoTime).getTime()) / 1000));
  if (diff < 60) {
    return `${diff}s 前`;
  }
  if (diff < 3600) {
    return `${Math.floor(diff / 60)}m 前`;
  }
  return `${Math.floor(diff / 3600)}h 前`;
}

export function normalizeMetricPoint(point?: MetricPoint | null) {
  if (!point) {
    return null;
  }

  const ts = new Date(point.ts).getTime();
  const value = Number(point.value || 0);
  if (!Number.isFinite(ts) || !Number.isFinite(value)) {
    return null;
  }

  return {
    ts: new Date(ts).toISOString(),
    value,
  };
}

export function mergeLatestHistory(
  current: HostHistoryMap | undefined,
  latestPoints: MetricLatestMap | undefined,
  windowSec: number,
) {
  const nextHistory: HostHistoryMap = { ...(current || {}) };
  const cutoff = Date.now() - windowSec * 1000;

  for (const metric of METRICS) {
    const point = normalizeMetricPoint(latestPoints?.[metric.key]);
    if (!point) {
      continue;
    }

    const merged = [ ...(nextHistory[metric.key] || []) ]
      .filter((existing) => new Date(existing.ts).getTime() !== new Date(point.ts).getTime());

    merged.push(point);
    merged.sort((left, right) => new Date(left.ts).getTime() - new Date(right.ts).getTime());

    nextHistory[metric.key] = merged.filter((existing) => {
      const ts = new Date(existing.ts).getTime();
      return Number.isFinite(ts) && ts >= cutoff;
    });
  }

  return nextHistory;
}

export function getWindowPoints(
  history: HostHistoryMap | undefined,
  metricKey: MetricKey,
  windowSec: number,
) {
  const raw = history?.[metricKey] || [];
  const cutoff = Date.now() - windowSec * 1000;

  return raw
    .map((point) => ({
      ts: new Date(point.ts).getTime(),
      value: Number(point.value || 0),
    }))
    .filter((point) => Number.isFinite(point.ts) && point.ts >= cutoff)
    .sort((left, right) => left.ts - right.ts);
}

export function currentWindowLabel(windowSec: number) {
  const found = WINDOW_OPTIONS.find((option) => option.seconds === windowSec);
  return found ? found.label : `${windowSec}s`;
}

export function stateLabel(stateCode?: number) {
  return STATE_LABELS[stateCode || 0] || "UNKNOWN";
}

export function stateClass(stateCode?: number) {
  return STATE_CLASSES[stateCode || 0] || "unknown";
}

