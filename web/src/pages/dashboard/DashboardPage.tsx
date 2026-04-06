import { startTransition, useEffect, useState } from "react";
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
  stateClass,
  stateLabel,
  sortHosts,
  WINDOW_OPTIONS,
  type HostDeletePayload,
  type HostHistoryMap,
  type HostSnapshot,
  type HostSyncPayload,
  type HostUpsertPayload,
  type MetricKey,
} from "./dashboard";

const HISTORY_RETENTION_SEC = 3600;

export function DashboardPage() {
  const { config } = useAppConfig();
  const { tenantCode } = useTenant();

  const [agents, setAgents] = useState<Record<string, HostSnapshot>>({});
  const [histories, setHistories] = useState<Record<string, HostHistoryMap>>({});
  const [selectedWindowSec, setSelectedWindowSec] = useState(300);
  const [selectedHostUID, setSelectedHostUID] = useState("");
  const [streamState, setStreamState] = useState("连接中");
  const [streamEventCount, setStreamEventCount] = useState(0);
  const [lastUpdated, setLastUpdated] = useState("");

  useEffect(() => {
    let cancelled = false;

    async function loadHosts() {
      try {
        const response = await fetch(`${config.apiBaseUrl}/hosts`);
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
  }, [config.apiBaseUrl]);

  useEffect(() => {
    const stream = new EventSource(config.streamPath);

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
          Object.entries(payload.latest || {}).forEach(([hostUID, latestPoints]) => {
            next[hostUID] = mergeLatestHistory(undefined, latestPoints, HISTORY_RETENTION_SEC);
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
  }, [config.streamPath]);

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
  const onlineCount = sortedHosts.filter((item) => item.overall_state !== 4).length;
  const offlineCount = sortedHosts.filter((item) => item.overall_state === 4).length;
  const avgCPU = sortedHosts.length
    ? sortedHosts.reduce((sum, item) => sum + Number(item.cpu_usage_pct || 0), 0) / sortedHosts.length
    : 0;
  const avgMem = sortedHosts.length
    ? sortedHosts.reduce((sum, item) => sum + Number(item.mem_used_pct || 0), 0) / sortedHosts.length
    : 0;
  const selectedSamples = selectedHost
    ? METRICS.reduce(
      (sum, metric) => sum + getWindowPoints(selectedHistory, metric.key, selectedWindowSec).length,
      0,
    )
    : 0;

  return (
    <div className="dashboard">


      <section className="dashboard-grid">
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
                : "选择左侧 Agent 查看趋势"}
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

            {selectedHost ? (
              <div className="window-meta">
                <span className={`status-pill ${stateClass(selectedHost.overall_state)}`}>
                  {stateLabel(selectedHost.overall_state)}
                </span>
                <span className="meta-pill">最后指标: {formatAgo(selectedHost.last_metric_at)}</span>
              </div>
            ) : null}
          </div>

          {!selectedHost ? (
            <div className="empty-state">当前没有可展示的 Agent 指标数据。</div>
          ) : (
            <div className="metric-grid">
              {METRICS.map((metric) => {
                const points = getWindowPoints(selectedHistory, metric.key, selectedWindowSec);
                const fallback = metric.source(selectedHost);
                const latest = points.length ? points[points.length - 1].value : fallback;
                const peak = points.length ? Math.max(...points.map((point) => point.value)) : fallback;
                const avg = points.length
                  ? points.reduce((sum, point) => sum + point.value, 0) / points.length
                  : fallback;

                return (
                  <article key={metric.key} className="metric-panel">
                    <div className="metric-panel-head">
                      <strong>{metric.label}</strong>
                      <span>{currentWindowLabel(selectedWindowSec)}</span>
                    </div>

                    <div className="metric-figures">
                      <MetricValue label="当前值" value={formatMetricValue(metric.key, latest)} />
                      <MetricValue label="窗口峰值" value={formatMetricValue(metric.key, peak)} />
                      <MetricValue label="窗口均值" value={formatMetricValue(metric.key, avg)} />
                    </div>

                    <MetricChart
                      metricKey={metric.key}
                      points={points}
                      emptyText={`${currentWindowLabel(selectedWindowSec)} 内还没有 ${metric.label} 样本`}
                    />
                  </article>
                );
              })}
            </div>
          )}
        </section>

      </section>      <section className="hero">
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

function MetricChart(props: { metricKey: MetricKey; points: Array<{ ts: number; value: number }>; emptyText: string }) {
  if (props.points.length < 2) {
    return <div className="chart-empty">{props.emptyText}</div>;
  }

  const width = 320;
  const height = 120;
  const padX = 12;
  const padY = 10;
  const plotWidth = width - padX * 2;
  const plotHeight = height - padY * 2;
  const minTs = props.points[0].ts;
  const maxTs = props.points[props.points.length - 1].ts;
  const maxValue = Math.max(1, ...props.points.map((point) => point.value)) * 1.15;
  const rangeTs = Math.max(1, maxTs - minTs);

  const linePoints = props.points
    .map((point) => {
      const x = padX + ((point.ts - minTs) / rangeTs) * plotWidth;
      const y = padY + (1 - point.value / maxValue) * plotHeight;
      return `${x.toFixed(2)},${y.toFixed(2)}`;
    })
    .join(" ");

  const endX = padX + ((props.points[props.points.length - 1].ts - minTs) / rangeTs) * plotWidth;
  const areaPath = `M ${padX} ${padY + plotHeight} L ${linePoints.split(" ").join(" L ")} L ${endX} ${padY + plotHeight} Z`;

  return (
    <div className="chart-frame">
      <svg viewBox={`0 0 ${width} ${height}`} preserveAspectRatio="none" aria-label={`${props.metricKey} history`}>
        <path className="chart-area" d={areaPath} />
        <polyline className="chart-line" points={linePoints} />
      </svg>
    </div>
  );
}
