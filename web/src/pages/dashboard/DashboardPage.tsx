import { startTransition, useEffect, useMemo, useState } from "react";
import { useAppConfig } from "../../app/providers/AppConfigProvider";
import { useTenant } from "../../app/providers/TenantProvider";
import {
  currentWindowLabel,
  formatAgo,
  formatMetricValue,
  formatPercent,
  getWindowPoints,
  listToHostMap,
  mergeLatestHistory,
  METRICS,
  normalizeHistoryMap,
  stateClass,
  stateLabel,
  sortHosts,
  WINDOW_OPTIONS,
  type MetricDefinition,
  type HostDeletePayload,
  type HostHistoryMap,
  type HostSnapshot,
  type HostSyncPayload,
  type HostUpsertPayload,
  type MetricKey,
} from "./dashboard";

const HISTORY_RETENTION_SEC = 3600;

type MetricCard = {
  key: string;
  label: string;
  series: MetricDefinition[];
};

type MetricSeriesStats = {
  key: MetricKey;
  label: string;
  points: Array<{ ts: number; value: number }>;
  latest: number;
  peak: number;
  avg: number;
};

const MERGED_METRIC_PAIRS: Array<{ label: string; keys: [MetricKey, MetricKey] }> = [
  { label: "磁盘吞吐", keys: ["disk_read_bps", "disk_write_bps"] },
  { label: "网络吞吐", keys: ["net_rx_bps", "net_tx_bps"] },
];

const DEFAULT_VISIBLE_WINDOW_METRICS: MetricKey[] = [
  "cpu_usage_pct",
  "mem_used_pct",
  "net_rx_bps",
  "net_tx_bps",
  "disk_read_bps",
  "disk_write_bps",
];

function metricTag(metricKey: MetricKey) {
  switch (metricKey) {
    case "disk_read_bps":
    case "disk_read_iops":
      return "读";
    case "disk_write_bps":
    case "disk_write_iops":
      return "写";
    case "net_rx_bps":
    case "net_rx_packets_ps":
      return "RX";
    case "net_tx_bps":
    case "net_tx_packets_ps":
      return "TX";
    default:
      return "";
  }
}

function metricBaseLabel(label: string, tag: string) {
  if (!tag) {
    return label;
  }
  return label.replace(new RegExp(`\\s*${tag}$`), "").replace(/读$|写$/, "");
}

function buildMetricCards(metrics: MetricDefinition[]) {
  const metricMap = new Map(metrics.map((metric) => [metric.key, metric]));
  const consumed = new Set<MetricKey>();
  const cards: MetricCard[] = [];

  for (const metric of metrics) {
    if (consumed.has(metric.key)) {
      continue;
    }

    const pair = MERGED_METRIC_PAIRS.find((item) => item.keys.includes(metric.key));
    if (pair) {
      const series = pair.keys
        .map((key) => metricMap.get(key))
        .filter((item): item is MetricDefinition => Boolean(item));
      if (series.length > 1) {
        series.forEach((item) => consumed.add(item.key));
        cards.push({
          key: pair.label,
          label: pair.label,
          series,
        });
        continue;
      }
    }

    consumed.add(metric.key);
    cards.push({
      key: metric.key,
      label: metric.label,
      series: [metric],
    });
  }

  return cards;
}

function buildTenantScopedUrl(path: string, tenantCode: string) {
  const url = new URL(path, window.location.origin);
  url.searchParams.set("tenant", tenantCode);
  return url.toString();
}

export function DashboardPage() {
  const { config, updateConfig } = useAppConfig();
  const { tenantCode } = useTenant();

  const [agents, setAgents] = useState<Record<string, HostSnapshot>>({});
  const [histories, setHistories] = useState<Record<string, HostHistoryMap>>({});
  const [selectedWindowSec, setSelectedWindowSec] = useState(300);
  const [selectedHostUID, setSelectedHostUID] = useState("");
  const [streamState, setStreamState] = useState("连接中");
  const [streamEventCount, setStreamEventCount] = useState(0);
  const [lastUpdated, setLastUpdated] = useState("");

  const hostsUrl = useMemo(
    () => buildTenantScopedUrl(`${config.apiBaseUrl}/hosts`, tenantCode),
    [config.apiBaseUrl, tenantCode],
  );
  const streamUrl = useMemo(
    () => buildTenantScopedUrl(config.streamPath, tenantCode),
    [config.streamPath, tenantCode],
  );

  useEffect(() => {
    startTransition(() => {
      setAgents({});
      setHistories({});
      setSelectedHostUID("");
      setStreamState("连接中");
      setStreamEventCount(0);
      setLastUpdated("");
    });
  }, [tenantCode]);

  useEffect(() => {
    let cancelled = false;

    async function loadHosts() {
      try {
        const response = await fetch(hostsUrl);
        if (!response.ok) {
          return;
        }

        const payload = (await response.json()) as { items?: HostSnapshot[] };
        if (cancelled) {
          return;
        }

        startTransition(() => {
          setAgents(listToHostMap(payload.items || []));
        });
      } catch {
        // The dashboard can still bootstrap from SSE sync.
      }
    }

    void loadHosts();

    return () => {
      cancelled = true;
    };
  }, [hostsUrl]);

  useEffect(() => {
    const stream = new EventSource(streamUrl);

    stream.addEventListener("open", () => {
      setStreamState("实时推送中");
    });

    stream.addEventListener("error", () => {
      setStreamState("重连中");
    });

    stream.addEventListener("sync", (event) => {
      const payload = JSON.parse((event as MessageEvent<string>).data) as HostSyncPayload;
      startTransition(() => {
        setStreamEventCount((current) => current + 1);
        setAgents(listToHostMap(payload.items || []));
        setHistories(() => {
          const next: Record<string, HostHistoryMap> = {};
          Object.entries(payload.histories || {}).forEach(([hostUID, history]) => {
            next[hostUID] = normalizeHistoryMap(history, HISTORY_RETENTION_SEC);
          });
          Object.entries(payload.latest || {}).forEach(([hostUID, latestPoints]) => {
            next[hostUID] = mergeLatestHistory(
              next[hostUID],
              latestPoints,
              HISTORY_RETENTION_SEC,
            );
          });
          return next;
        });
        setLastUpdated(payload.server_time || "");
      });
    });

    stream.addEventListener("host_upsert", (event) => {
      const payload = JSON.parse((event as MessageEvent<string>).data) as HostUpsertPayload;
      if (!payload.item?.host_uid) {
        return;
      }

      startTransition(() => {
        setStreamEventCount((current) => current + 1);
        setAgents((current) => ({
          ...current,
          [payload.item!.host_uid]: payload.item!,
        }));
        setHistories((current) => ({
          ...current,
          [payload.item!.host_uid]: mergeLatestHistory(
            current[payload.item!.host_uid],
            payload.latest,
            HISTORY_RETENTION_SEC,
          ),
        }));
        setLastUpdated(payload.server_time || "");
      });
    });

    stream.addEventListener("host_delete", (event) => {
      const payload = JSON.parse((event as MessageEvent<string>).data) as HostDeletePayload;
      if (!payload.host_uid) {
        return;
      }

      startTransition(() => {
        setStreamEventCount((current) => current + 1);
        setAgents((current) => {
          const next = { ...current };
          delete next[payload.host_uid!];
          return next;
        });
        setHistories((current) => {
          const next = { ...current };
          delete next[payload.host_uid!];
          return next;
        });
        setLastUpdated(payload.server_time || "");
      });
    });

    return () => {
      stream.close();
    };
  }, [streamUrl]);

  const sortedHosts = sortHosts(Object.values(agents));

  useEffect(() => {
    if (!sortedHosts.length) {
      if (selectedHostUID) {
        setSelectedHostUID("");
      }
      return;
    }

    if (!selectedHostUID || !agents[selectedHostUID]) {
      setSelectedHostUID(sortedHosts[0].host_uid);
    }
  }, [agents, selectedHostUID, sortedHosts]);

  const selectedHost = selectedHostUID ? agents[selectedHostUID] : undefined;
  const selectedHistory = selectedHostUID ? histories[selectedHostUID] : undefined;
  const visibleMetrics = useMemo(() => {
    const metricMap = new Map(METRICS.map((metric) => [metric.key, metric]));
    return config.visibleWindowMetrics
      .map((key) => metricMap.get(key))
      .filter((metric): metric is (typeof METRICS)[number] => Boolean(metric));
  }, [config.visibleWindowMetrics]);
  const visibleMetricCards = useMemo(() => buildMetricCards(visibleMetrics), [visibleMetrics]);
  const onlineCount = sortedHosts.filter((item) => item.overall_state !== 4).length;
  const offlineCount = sortedHosts.filter((item) => item.overall_state === 4).length;
  const avgCPU = sortedHosts.length
    ? sortedHosts.reduce((sum, item) => sum + Number(item.cpu_usage_pct || 0), 0) / sortedHosts.length
    : 0;
  const avgMem = sortedHosts.length
    ? sortedHosts.reduce((sum, item) => sum + Number(item.mem_used_pct || 0), 0) / sortedHosts.length
    : 0;
  const selectedSamples = selectedHost
    ? visibleMetrics.reduce(
      (sum, metric) => sum + getWindowPoints(selectedHistory, metric.key, selectedWindowSec).length,
      0,
    )
    : 0;

  function toggleVisibleMetric(metricKey: MetricKey) {
    const current = config.visibleWindowMetrics;
    const exists = current.includes(metricKey);
    if (exists && current.length <= 1) {
      return;
    }

    updateConfig({
      visibleWindowMetrics: exists
        ? current.filter((key) => key !== metricKey)
        : [...current, metricKey],
    });
  }

  function resetVisibleMetrics() {
    updateConfig({
      visibleWindowMetrics: DEFAULT_VISIBLE_WINDOW_METRICS,
    });
  }

  return (
    <div className="dashboard">
      <section className="panel">
        <div className="panel-head">
          <div>
            <div className="eyebrow">Agents</div>
            <h2>主机概览</h2>
          </div>
          <p className="panel-note">点击卡片查看该 Agent 的窗口指标。</p>
        </div>

        <div className="host-list">
          {!sortedHosts.length ? (
            <div className="empty-state">等待 Agent 上报...</div>
          ) : (
            sortedHosts.map((item) => (
              <button
                key={item.host_uid}
                type="button"
                className={`host-card ${item.host_uid === selectedHostUID ? "selected" : ""}`}
                onClick={() => setSelectedHostUID(item.host_uid)}
              >
                <div className="host-card-head">
                  <div>
                    <strong>{item.hostname || item.host_uid}</strong>
                    <span>{item.primary_ip || item.host_uid}</span>
                  </div>
                  <span className={`status-pill ${stateClass(item.overall_state)}`}>
                    {stateLabel(item.overall_state)}
                  </span>
                </div>

                <div className="host-card-metrics">
                  <MetricValue label="CPU" value={formatMetricValue("cpu_usage_pct", Number(item.cpu_usage_pct || 0))} />
                  <MetricValue label="内存" value={formatMetricValue("mem_used_pct", Number(item.mem_used_pct || 0))} />
                  <MetricValue label="磁盘" value={formatMetricValue("disk_used_pct", Number(item.disk_used_pct || 0))} />
                  <MetricValue label="负载" value={formatMetricValue("load1", Number(item.load1 || 0))} />
                </div>

                <div className="host-card-foot">
                  <span>最后心跳: {formatAgo(item.last_agent_seen_at)}</span>
                  <span>版本: {item.version || 0}</span>
                </div>
              </button>
            ))
          )}
        </div>
      </section>

      <section className="panel">
        <div className="panel-head">
          <div>
            <div className="eyebrow">Window</div>
            <h2>{selectedHost ? selectedHost.hostname || selectedHost.host_uid : "窗口详情"}</h2>
          </div>
          <p className="panel-note">
            {selectedHost
              ? `${currentWindowLabel(selectedWindowSec)} 内累计 ${selectedSamples} 个样本`
              : "选择上方 Agent 查看趋势"}
          </p>
        </div>

        <div className="window-toolbar">
          <div className="window-group">
            {WINDOW_OPTIONS.map((option) => (
              <button
                key={option.seconds}
                type="button"
                className={`chip ${option.seconds === selectedWindowSec ? "active" : ""}`}
                onClick={() => setSelectedWindowSec(option.seconds)}
              >
                {option.label}
              </button>
            ))}
          </div>

          <div className="window-side">
            <details className="window-config">
              <summary className="chip">
                展示项 {visibleMetrics.length}/{METRICS.length}
              </summary>
              <div className="window-config-panel">
                <div className="window-config-grid">
                  {METRICS.map((metric) => {
                    const active = config.visibleWindowMetrics.includes(metric.key);
                    return (
                      <button
                        key={metric.key}
                        type="button"
                        className={`chip metric-toggle ${active ? "active" : ""}`}
                        onClick={() => toggleVisibleMetric(metric.key)}
                      >
                        {metric.label}
                      </button>
                    );
                  })}
                </div>
                <button type="button" className="chip window-config-reset" onClick={resetVisibleMetrics}>
                  恢复默认前 6 项
                </button>
              </div>
            </details>

            {selectedHost ? (
              <div className="window-meta">
              <span className={`status-pill ${stateClass(selectedHost.overall_state)}`}>
                {stateLabel(selectedHost.overall_state)}
              </span>
              <span className="meta-pill">最后指标: {formatAgo(selectedHost.last_metric_at)}</span>
              </div>
            ) : null}
          </div>
        </div>

        {!selectedHost ? (
          <div className="empty-state">当前没有可展示的 Agent 指标数据。</div>
        ) : (
          <div
            className="metric-grid"
            style={{ ["--metric-count" as string]: String(visibleMetricCards.length || 1) }}
          >
            {visibleMetricCards.map((card) => {
              const seriesStats: MetricSeriesStats[] = card.series.map((metric) => {
                const points = getWindowPoints(selectedHistory, metric.key, selectedWindowSec);
                const fallback = metric.source(selectedHost);
                return {
                  key: metric.key,
                  label: metric.label,
                  points,
                  latest: points.length ? points[points.length - 1].value : fallback,
                  peak: points.length ? Math.max(...points.map((point) => point.value)) : fallback,
                  avg: points.length
                    ? points.reduce((sum, point) => sum + point.value, 0) / points.length
                    : fallback,
                };
              });

              return (
                <article key={card.key} className="metric-panel">
                  <div className="metric-panel-head">
                    <strong>{card.label}</strong>
                    <span>{currentWindowLabel(selectedWindowSec)}</span>
                  </div>

                  <div className={`metric-figures ${seriesStats.length > 1 ? "dual-series" : ""}`}>
                    {seriesStats.map((series) => (
                      <MetricSeriesValue
                        key={series.key}
                        label={series.label}
                        metricKey={series.key}
                        latest={series.latest}
                        peak={series.peak}
                        avg={series.avg}
                      />
                    ))}
                  </div>

                  <MetricChart
                    label={card.label}
                    series={seriesStats.map((series) => ({
                      key: series.key,
                      label: series.label,
                      points: series.points,
                    }))}
                    emptyText={`${currentWindowLabel(selectedWindowSec)} 内还没有 ${card.label} 样本`}
                  />
                </article>
              );
            })}
          </div>
        )}
      </section>

      <section className="hero">
        <div className="hero-copy">
          <div className="eyebrow">Live Dashboard</div>
          <h2 className="hero-title">核心指标与窗口信息</h2>
          <p>
            单页面 React 看板当前直接复用 `master-api` 的 SSE 数据流，聚焦主机状态、核心指标和最近窗口趋势，
            同时保留对 H5 的响应式展示。
          </p>
        </div>

        <div className="hero-meta">
          <span className="meta-pill">Tenant: {tenantCode}</span>
          <span className="meta-pill">推送状态: {streamState}</span>
          <span className="meta-pill">流事件: {streamEventCount}</span>
          <span className="meta-pill">
            最近刷新: {lastUpdated ? new Date(lastUpdated).toLocaleTimeString() : "--"}
          </span>
        </div>
      </section>

      <section className="stats-grid">
        <StatCard label="Agent 总数" value={String(sortedHosts.length)} />
        <StatCard label="在线 Agent" value={String(onlineCount)} />
        <StatCard label="离线 Agent" value={String(offlineCount)} />
        <StatCard label="平均 CPU" value={formatPercent(avgCPU)} />
        <StatCard label="平均内存" value={formatPercent(avgMem)} />
        <StatCard label="当前窗口" value={currentWindowLabel(selectedWindowSec)} />
      </section>
    </div>
  );
}

function StatCard(props: { label: string; value: string }) {
  return (
    <article className="stat-card">
      <span>{props.label}</span>
      <strong>{props.value}</strong>
    </article>
  );
}

function MetricValue(props: { label: string; value: string }) {
  return (
    <div className="metric-value">
      <span>{props.label}</span>
      <strong>{props.value}</strong>
    </div>
  );
}

function MetricSeriesValue(props: {
  label: string;
  metricKey: MetricKey;
  latest: number;
  peak: number;
  avg: number;
}) {
  const tag = metricTag(props.metricKey);
  const baseLabel = metricBaseLabel(props.label, tag);
  return (
    <div className="metric-series">
      <div className="metric-series-head">
        <strong>{baseLabel}</strong>
        {tag ? <span className="metric-tag">{tag}</span> : null}
      </div>
      <span>当前 {formatMetricValue(props.metricKey, props.latest)}</span>
      <span>峰值 {formatMetricValue(props.metricKey, props.peak)}</span>
      <span>均值 {formatMetricValue(props.metricKey, props.avg)}</span>
    </div>
  );
}

function MetricChart(props: {
  label: string;
  series: Array<{ key: MetricKey; label: string; points: Array<{ ts: number; value: number }> }>;
  emptyText: string;
}) {
  const seriesWithPoints = props.series.filter((item) => item.points.length >= 2);
  if (!seriesWithPoints.length) {
    return <div className="chart-empty">{props.emptyText}</div>;
  }

  const width = 240;
  const height = 88;
  const padX = 8;
  const padY = 8;
  const plotWidth = width - padX * 2;
  const plotHeight = height - padY * 2;
  const allPoints = seriesWithPoints.flatMap((item) => item.points);
  const minTs = Math.min(...allPoints.map((point) => point.ts));
  const maxTs = Math.max(...allPoints.map((point) => point.ts));
  const maxValue = Math.max(1, ...allPoints.map((point) => point.value)) * 1.15;
  const rangeTs = Math.max(1, maxTs - minTs);
  const colors = ["#0f766e", "#b7791f", "#c2410c", "#2563eb"];
  const directional = seriesWithPoints.length === 2;
  const centerY = padY + plotHeight / 2;

  return (
    <div className="chart-frame">
      <svg viewBox={`0 0 ${width} ${height}`} preserveAspectRatio="none" aria-label={`${props.label} history`}>
        {directional ? (
          <line
            className="chart-midline"
            x1={padX}
            y1={centerY}
            x2={padX + plotWidth}
            y2={centerY}
          />
        ) : null}
        {seriesWithPoints.map((item, index) => {
          const directionalMax = Math.max(1, ...item.points.map((point) => point.value)) * 1.15;
          const linePoints = item.points
            .map((point) => {
              const x = padX + ((point.ts - minTs) / rangeTs) * plotWidth;
              let y = padY + (1 - point.value / maxValue) * plotHeight;
              if (directional) {
                const halfHeight = plotHeight / 2;
                if (index === 0) {
                  y = centerY - (point.value / directionalMax) * halfHeight;
                } else {
                  y = centerY + (point.value / directionalMax) * halfHeight;
                }
              }
              return `${x.toFixed(2)},${y.toFixed(2)}`;
            })
            .join(" ");

          return (
            <polyline
              key={item.key}
              className="chart-line"
              points={linePoints}
              style={{ stroke: colors[index % colors.length] }}
            />
          );
        })}
      </svg>
    </div>
  );
}
