import { useState, type CSSProperties } from "react";
import { Link, useParams } from "react-router-dom";
import { useAppConfig } from "../../app/providers/AppConfigProvider";
import {
  currentWindowLabel,
  formatAgo,
  formatBytes,
  formatMetricValue,
  getWindowPoints,
  stateClass,
  stateLabel,
  WINDOW_OPTIONS,
  type HostHistoryMap,
  type HostSnapshot,
  type MetricKey,
} from "../dashboard/dashboard";
import { useLiveHostsData } from "../dashboard/useLiveHostsData";

type GaugeTone = "healthy" | "warning" | "critical";

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value));
}

function getLoadProgress(host: HostSnapshot) {
  return clamp(Number(host.load1 || 0) * 24, 0, 100);
}

function getGaugeTone(value: number): GaugeTone {
  if (value >= 85) {
    return "critical";
  }
  if (value >= 65) {
    return "warning";
  }
  return "healthy";
}

function getThresholdTone(value: number): GaugeTone {
  if (value >= 80) {
    return "warning";
  }
  return "healthy";
}

function getHostTone(host: HostSnapshot) {
  if (host.overall_state === 3 || host.overall_state === 4) {
    return "critical";
  }
  if (host.overall_state === 2) {
    return "warning";
  }
  return "healthy";
}

function readHostMetric(host: HostSnapshot, metricKey: MetricKey) {
  return Number(host[metricKey as keyof HostSnapshot] || 0);
}

function metricAverage(points: Array<{ ts: number; value: number }>) {
  if (!points.length) {
    return 0;
  }
  return points.reduce((sum, item) => sum + item.value, 0) / points.length;
}

function estimateTotalBytes(freeBytes: number, usedPercent: number) {
  const ratio = clamp(usedPercent / 100, 0, 0.99);
  if (freeBytes <= 0) {
    return 0;
  }
  if (ratio <= 0) {
    return freeBytes;
  }
  return freeBytes / (1 - ratio);
}

function formatVersion(version?: number) {
  return version ? `v${version}` : "v0";
}

function metricShare(primary: number, secondary: number) {
  const total = primary + secondary;
  if (total <= 0) {
    return 50;
  }
  return clamp((primary / total) * 100, 0, 100);
}

function samplePoints(
  points: Array<{ ts: number; value: number }>,
  maxPoints: number,
) {
  if (points.length <= maxPoints) {
    return points;
  }

  const next: Array<{ ts: number; value: number }> = [];
  const lastIndex = points.length - 1;
  const step = lastIndex / (maxPoints - 1);

  for (let index = 0; index < maxPoints; index += 1) {
    const sourceIndex = Math.min(lastIndex, Math.round(index * step));
    next.push(points[sourceIndex]);
  }

  return next;
}

export function MobileAgentPage() {
  const { config } = useAppConfig();
  const { tenantCode = "default", hostUID } = useParams();
  const [selectedWindowSec, setSelectedWindowSec] = useState(300);
  const { histories, lastUpdated, sortedHosts, streamEventCount, streamState } = useLiveHostsData({
    apiBaseUrl: config.apiBaseUrl,
    streamPath: config.streamPath,
    tenantCode,
  });

  const topTime = new Intl.DateTimeFormat("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date());
  const selectedHost = hostUID
    ? sortedHosts.find((item) => item.host_uid === hostUID)
    : undefined;
  const isDetailView = Boolean(hostUID);

  return (
    <div className={`mobile-pwa ${isDetailView ? "detail-view" : "list-view"}`}>
      <div className={`pwa-phone-shell ${isDetailView ? "detail-view" : "list-view"}`}>
        <header className="pwa-statusbar">
          <span>{topTime}</span>
          <div className="pwa-statusbar-icons" aria-hidden="true">
            <i className="pwa-signal-icon" />
            <i className="pwa-battery-icon" />
          </div>
        </header>

        {isDetailView ? (
          <PwaDetailPage
            host={selectedHost}
            history={hostUID ? histories[hostUID] : undefined}
            lastUpdated={lastUpdated}
            selectedWindowSec={selectedWindowSec}
            setSelectedWindowSec={setSelectedWindowSec}
            streamState={streamState}
            tenantCode={tenantCode}
          />
        ) : (
          <PwaStatusPage
            lastUpdated={lastUpdated}
            selectedWindowSec={selectedWindowSec}
            setSelectedWindowSec={setSelectedWindowSec}
            sortedHosts={sortedHosts}
            histories={histories}
            streamEventCount={streamEventCount}
            streamState={streamState}
            tenantCode={tenantCode}
          />
        )}

        <nav className="pwa-tabbar" aria-label="Prototype navigation">
          {WINDOW_OPTIONS.map((option) => (
            <button
              key={option.seconds}
              type="button"
              className={`pwa-tab-window ${option.seconds === selectedWindowSec ? "active" : ""}`}
              onClick={() => setSelectedWindowSec(option.seconds)}
            >
              {option.label}
            </button>
          ))}
        </nav>
      </div>
    </div>
  );
}

function PwaStatusPage(props: {
  histories: Record<string, HostHistoryMap>;
  lastUpdated: string;
  selectedWindowSec: number;
  setSelectedWindowSec: (value: number) => void;
  sortedHosts: HostSnapshot[];
  streamEventCount: number;
  streamState: string;
  tenantCode: string;
}) {
  const {
    histories,
    lastUpdated,
    selectedWindowSec,
    setSelectedWindowSec,
    sortedHosts,
    streamEventCount,
    streamState,
    tenantCode,
  } = props;

  const summary = {
    total: sortedHosts.length,
    online: sortedHosts.filter((item) => item.overall_state !== 4).length,
    avgCPU: sortedHosts.length
      ? sortedHosts.reduce((sum, item) => sum + Number(item.cpu_usage_pct || 0), 0) / sortedHosts.length
      : 0,
    avgMem: sortedHosts.length
      ? sortedHosts.reduce((sum, item) => sum + Number(item.mem_used_pct || 0), 0) / sortedHosts.length
      : 0,
  };
  const hostColumns = sortedHosts.length > 4 ? 2 : 1;
  const hostRows = Math.max(1, Math.ceil(sortedHosts.length / hostColumns));

  return (
    <>
      <section className="pwa-hero-card">
        <div className="eyebrow">Tenant · {tenantCode}</div>
        <h1>Overview</h1>
        <p>点击设备进入详情页。移动端原型继续复用当前 Agent 实时流和现有配色体系。</p>

        <div className="pwa-summary-grid">
          <SummaryCell label="在线" value={`${summary.online}/${summary.total || 0}`} />
          <SummaryCell label="平均 CPU" value={formatMetricValue("cpu_usage_pct", summary.avgCPU)} />
          <SummaryCell label="平均内存" value={formatMetricValue("mem_used_pct", summary.avgMem)} />
          <SummaryCell label="推送" value={streamState} />
        </div>

        <div className="pwa-summary-foot">
          <span>流事件 {streamEventCount}</span>
          <span>最近刷新 {lastUpdated ? new Date(lastUpdated).toLocaleTimeString() : "--"}</span>
        </div>
      </section>

      <main
        className="pwa-host-list"
        style={
          {
            ["--host-count" as string]: String(Math.max(1, sortedHosts.length)),
            ["--host-columns" as string]: String(hostColumns),
            ["--host-rows" as string]: String(hostRows),
          } as CSSProperties
        }
      >
        {!sortedHosts.length ? (
          <div className="pwa-empty-state">等待 Agent 上报...</div>
        ) : (
          sortedHosts.map((host) => (
            <HostStatusCard
              key={host.host_uid}
              history={histories[host.host_uid]}
              host={host}
              selectedWindowSec={selectedWindowSec}
              tenantCode={tenantCode}
            />
          ))
        )}
      </main>
    </>
  );
}

function PwaDetailPage(props: {
  host: HostSnapshot | undefined;
  history: HostHistoryMap | undefined;
  lastUpdated: string;
  selectedWindowSec: number;
  setSelectedWindowSec: (value: number) => void;
  streamState: string;
  tenantCode: string;
}) {
  const {
    host,
    history,
    lastUpdated,
    selectedWindowSec,
    setSelectedWindowSec,
    streamState,
    tenantCode,
  } = props;

  if (!host) {
    return (
      <section className="pwa-detail-shell">
        <Link to={`/${tenantCode}/pwa`} className="pwa-backlink">
          <span className="pwa-back-icon" aria-hidden="true" />
          <span>返回</span>
        </Link>
        <div className="pwa-empty-state">当前设备不存在，或数据还未同步完成。</div>
      </section>
    );
  }

  const cpuPoints = getWindowPoints(history, "cpu_usage_pct", selectedWindowSec);
  const memPoints = getWindowPoints(history, "mem_used_pct", selectedWindowSec);
  const netRxPoints = getWindowPoints(history, "net_rx_bps", selectedWindowSec);
  const netTxPoints = getWindowPoints(history, "net_tx_bps", selectedWindowSec);
  const diskReadPoints = getWindowPoints(history, "disk_read_bps", selectedWindowSec);
  const diskWritePoints = getWindowPoints(history, "disk_write_bps", selectedWindowSec);
  const cpuUsage = readHostMetric(host, "cpu_usage_pct");
  const memUsage = readHostMetric(host, "mem_used_pct");
  const diskUsage = readHostMetric(host, "disk_used_pct");
  const loadProgress = getLoadProgress(host);
  const memFree = readHostMetric(host, "mem_available_bytes");
  const memTotal = estimateTotalBytes(memFree, memUsage);
  const memUsed = Math.max(0, memTotal - memFree);
  const diskFree = readHostMetric(host, "disk_free_bytes");
  const diskTotal = estimateTotalBytes(diskFree, diskUsage);
  const diskUsed = Math.max(0, diskTotal - diskFree);
  const netRx = readHostMetric(host, "net_rx_bps");
  const netTx = readHostMetric(host, "net_tx_bps");
  const diskRead = readHostMetric(host, "disk_read_bps");
  const diskWrite = readHostMetric(host, "disk_write_bps");
  const netBalance = metricShare(netRx, netTx);
  const diskBalance = metricShare(diskRead, diskWrite);

  return (
    <section className="pwa-detail-shell">
      <div className="pwa-detail-header">
        <Link to={`/${tenantCode}/pwa`} className="pwa-backlink">
          <span className="pwa-back-icon" aria-hidden="true" />
          <span>返回</span>
        </Link>

        <div className="pwa-detail-title">
          <h1>{host.hostname || host.host_uid}</h1>
          <p>
            {host.primary_ip || host.host_uid} · {formatVersion(host.version)} · {stateLabel(host.overall_state)}
          </p>
        </div>

      </div>

      <div className="pwa-detail-landscape-topbar">
        <Link to={`/${tenantCode}/pwa`} className="pwa-backlink compact">
          <span className="pwa-back-icon" aria-hidden="true" />
          <span>返回</span>
        </Link>
        <strong>{host.hostname || host.host_uid}</strong>
      </div>

      <main className="pwa-detail-grid">
        <section className="pwa-detail-card pwa-detail-card-cpu">
          <div className="pwa-detail-card-head">
            <div className="pwa-detail-big-metric">
              <strong>{Math.round(cpuUsage)}%</strong>
              <span>CPU</span>
            </div>

            <div className="pwa-detail-stat-row">
              <DetailStat label="状态" value={stateLabel(host.overall_state)} accent={stateClass(host.overall_state)} />
              <DetailStat label="Load" value={formatMetricValue("load1", readHostMetric(host, "load1"))} accent={getGaugeTone(loadProgress)} />
            </div>
          </div>

          <HistoryStrip points={cpuPoints} />

          <div className="pwa-detail-meta-grid">
            <DetailMeta label="内存占用" value={formatMetricValue("mem_used_pct", memUsage)} />
            <DetailMeta label="磁盘占用" value={formatMetricValue("disk_used_pct", diskUsage)} />
            <DetailMeta label="最后心跳" value={formatAgo(host.last_agent_seen_at)} />
            <div className="pwa-detail-side-gauge">
              <span>负载</span>
              <div className={`pwa-gauge tone-${getGaugeTone(loadProgress)}`}>
                <LoadRings value={loadProgress} />
              </div>
            </div>
          </div>
        </section>

        <section className="pwa-detail-card">
          <div className="pwa-detail-card-head compact">
            <div>
              <h2>Network</h2>
              <span className="pwa-detail-quiet">
                ↓ {formatMetricValue("net_rx_bps", netRx)} · ↑ {formatMetricValue("net_tx_bps", netTx)}
              </span>
            </div>
            <BalanceDonut value={netBalance} tone={getGaugeTone(netBalance)} />
          </div>

          <div className="pwa-detail-kv-grid">
            <DetailMeta label="Down" value={formatMetricValue("net_rx_bps", netRx)} />
            <DetailMeta label="Up" value={formatMetricValue("net_tx_bps", netTx)} />
            <DetailMeta label="RX pkt" value={formatMetricValue("net_rx_packets_ps", readHostMetric(host, "net_rx_packets_ps"))} />
            <DetailMeta label="TX pkt" value={formatMetricValue("net_tx_packets_ps", readHostMetric(host, "net_tx_packets_ps"))} />
          </div>

          <DualInsetTrend
            label="Network"
            topLabel="RX"
            bottomLabel="TX"
            topMetricKey="net_rx_bps"
            bottomMetricKey="net_tx_bps"
            topPoints={netRxPoints}
            bottomPoints={netTxPoints}
          />
        </section>

        <section className="pwa-detail-card">
          <div className="pwa-detail-card-head compact">
            <div>
              <h2>Disk</h2>
              <span className="pwa-detail-quiet">{formatMetricValue("disk_used_pct", diskUsage)}</span>
            </div>
            <div className={`pwa-storage-pill tone-${getThresholdTone(diskUsage)}`}>
              <span style={{ height: `${clamp(diskUsage, 0, 100)}%` }} />
            </div>
          </div>

          <div className="pwa-detail-kv-grid">
            <DetailMeta label="Used" value={formatBytes(diskUsed)} />
            <DetailMeta label="Free" value={formatBytes(diskFree)} />
            <DetailMeta label="Read" value={formatMetricValue("disk_read_bps", diskRead)} />
            <DetailMeta label="Write" value={formatMetricValue("disk_write_bps", diskWrite)} />
            <DetailMeta label="Read IOPS" value={formatMetricValue("disk_read_iops", readHostMetric(host, "disk_read_iops"))} />
            <DetailMeta label="Write IOPS" value={formatMetricValue("disk_write_iops", readHostMetric(host, "disk_write_iops"))} />
          </div>

          <DualInsetTrend
            label="Disk throughput"
            topLabel="Read"
            bottomLabel="Write"
            topMetricKey="disk_read_bps"
            bottomMetricKey="disk_write_bps"
            topPoints={diskReadPoints}
            bottomPoints={diskWritePoints}
          />
        </section>

        <section className="pwa-detail-card">
          <div className="pwa-detail-card-head compact">
            <div>
              <h2>Memory</h2>
              <span className="pwa-detail-quiet">
                {formatMetricValue("mem_used_pct", metricAverage(memPoints) || memUsage)}
              </span>
            </div>
            <div className={`pwa-gauge compact tone-${getGaugeTone(memUsage)}`}>
              <DonutGauge value={memUsage} />
            </div>
          </div>

          <div className="pwa-detail-kv-grid">
            <DetailMeta label="Free" value={formatBytes(memFree)} />
            <DetailMeta label="Used" value={formatBytes(memUsed)} />
            <DetailMeta label="Total" value={formatBytes(memTotal)} />
          </div>

          <InsetTrend
            label="Memory trend"
            primary={memPoints}
            metricKey="mem_used_pct"
          />
        </section>

        <section className="pwa-detail-card pwa-detail-card-meta">
          <div className="pwa-detail-card-head compact">
            <div>
              <h2>Device</h2>
              <span className="pwa-detail-quiet">{host.primary_ip || host.host_uid}</span>
            </div>
            <span className={`status-pill ${stateClass(host.overall_state)}`}>
              {stateLabel(host.overall_state)}
            </span>
          </div>

          <div className="pwa-detail-kv-grid">
            <DetailMeta label="Name" value={host.hostname || host.host_uid} />
            <DetailMeta label="Host UID" value={host.host_uid} />
            <DetailMeta label="Primary IP" value={host.primary_ip || "--"} />
            <DetailMeta label="Version" value={formatVersion(host.version)} />
            <DetailMeta label="Metric time" value={formatAgo(host.last_metric_at)} />
            <DetailMeta label="Heartbeat" value={formatAgo(host.last_agent_seen_at)} />
            <DetailMeta label="Stream" value={streamState} />
            <DetailMeta label="Updated" value={lastUpdated ? new Date(lastUpdated).toLocaleTimeString() : "--"} />
          </div>
        </section>
      </main>
    </section>
  );
}

function SummaryCell(props: { label: string; value: string }) {
  return (
    <article className="pwa-summary-cell">
      <span>{props.label}</span>
      <strong>{props.value}</strong>
    </article>
  );
}

function HostStatusCard(props: {
  history: HostHistoryMap | undefined;
  host: HostSnapshot;
  selectedWindowSec: number;
  tenantCode: string;
}) {
  const { history, host, selectedWindowSec, tenantCode } = props;
  const loadProgress = getLoadProgress(host);
  const memProgress = clamp(Number(host.mem_used_pct || 0), 0, 100);
  const loadTone = getGaugeTone(loadProgress);
  const memTone = getGaugeTone(memProgress);
  const hostTone = getHostTone(host);
  const cpuPoints = getWindowPoints(history, "cpu_usage_pct", selectedWindowSec);

  return (
    <Link
      to={`/${tenantCode}/pwa/${encodeURIComponent(host.host_uid)}`}
      className={`pwa-host-card tone-${hostTone}`}
    >
      <div className="pwa-host-head">
        <div>
          <h2>{host.hostname || host.host_uid}</h2>
          <p>{host.primary_ip || host.host_uid}</p>
        </div>

        <div className="pwa-host-head-side">
          <div className="pwa-host-card-topline">
            <span className={`status-pill ${stateClass(host.overall_state)}`}>
              {stateLabel(host.overall_state)}
            </span>
            <span className="pwa-host-action" aria-hidden="true" />
          </div>
          <span className="pwa-host-footnote">心跳 {formatAgo(host.last_agent_seen_at)}</span>
        </div>
      </div>

      <div className="pwa-metric-grid">
        <GaugeTile
          label="Load"
          caption={formatMetricValue("load1", Number(host.load1 || 0))}
          value={loadProgress}
          tone={loadTone}
          variant="rings"
        />
        <GaugeTile
          label="Mem"
          caption={formatMetricValue("mem_used_pct", memProgress)}
          value={memProgress}
          tone={memTone}
          variant="donut"
        />
        <RateTile
          title="网络"
          topLabel="RX"
          topValue={formatMetricValue("net_rx_bps", readHostMetric(host, "net_rx_bps"))}
          bottomLabel="TX"
          bottomValue={formatMetricValue("net_tx_bps", readHostMetric(host, "net_tx_bps"))}
        />
        <RateTile
          title="磁盘"
          topLabel="读"
          topValue={formatMetricValue("disk_read_bps", readHostMetric(host, "disk_read_bps"))}
          bottomLabel="写"
          bottomValue={formatMetricValue("disk_write_bps", readHostMetric(host, "disk_write_bps"))}
        />
      </div>

      <div className="pwa-host-footer">
        <div>
          <span className="pwa-host-footnote">
            窗口 {currentWindowLabel(selectedWindowSec)} · 指标 {formatAgo(host.last_metric_at)}
          </span>
          <div className="pwa-host-inline-metrics">
            <span>CPU {formatMetricValue("cpu_usage_pct", readHostMetric(host, "cpu_usage_pct"))}</span>
            <span>磁盘 {formatMetricValue("disk_used_pct", readHostMetric(host, "disk_used_pct"))}</span>
          </div>
        </div>
        <MiniSparkline points={cpuPoints} />
      </div>
    </Link>
  );
}

function DetailStat(props: { label: string; value: string; accent: string }) {
  return (
    <div className={`pwa-detail-stat accent-${props.accent}`}>
      <span>{props.label}</span>
      <strong>{props.value}</strong>
    </div>
  );
}

function DetailMeta(props: { label: string; value: string }) {
  return (
    <div className="pwa-detail-meta">
      <span>{props.label}</span>
      <strong>{props.value}</strong>
    </div>
  );
}

function GaugeTile(props: {
  label: string;
  caption: string;
  value: number;
  tone: GaugeTone;
  variant: "rings" | "donut";
}) {
  return (
    <div className="pwa-gauge-tile">
      <div className={`pwa-gauge tone-${props.tone}`}>
        {props.variant === "rings" ? <LoadRings value={props.value} /> : <DonutGauge value={props.value} />}
      </div>
      <div className="pwa-gauge-copy">
        <strong>{props.label}</strong>
        <span>{props.caption}</span>
      </div>
    </div>
  );
}

function LoadRings(props: { value: number }) {
  const rings = [36, 27, 18];
  return (
    <svg viewBox="0 0 88 88" aria-hidden="true">
      {rings.map((radius, index) => {
        const circumference = 2 * Math.PI * radius;
        const dashOffset = circumference * (1 - props.value / 100);
        return (
          <g key={radius}>
            <circle className="pwa-ring-track" cx="44" cy="44" r={radius} />
            <circle
              className={`pwa-ring-progress pwa-ring-progress-${index + 1}`}
              cx="44"
              cy="44"
              r={radius}
              strokeDasharray={circumference}
              strokeDashoffset={dashOffset}
            />
          </g>
        );
      })}
      <circle className="pwa-ring-core" cx="44" cy="44" r="10" />
    </svg>
  );
}

function DonutGauge(props: { value: number }) {
  const radius = 28;
  const circumference = 2 * Math.PI * radius;
  const dashOffset = circumference * (1 - props.value / 100);

  return (
    <svg viewBox="0 0 88 88" aria-hidden="true">
      <circle className="pwa-donut-track" cx="44" cy="44" r={radius} />
      <circle
        className="pwa-donut-progress"
        cx="44"
        cy="44"
        r={radius}
        strokeDasharray={circumference}
        strokeDashoffset={dashOffset}
      />
      <text x="44" y="49" textAnchor="middle">
        {Math.round(props.value)}%
      </text>
    </svg>
  );
}

function BalanceDonut(props: { value: number; tone: GaugeTone }) {
  return (
    <div className={`pwa-gauge compact tone-${props.tone}`}>
      <DonutGauge value={props.value} />
    </div>
  );
}

function RateTile(props: {
  title: string;
  topLabel: string;
  topValue: string;
  bottomLabel: string;
  bottomValue: string;
}) {
  return (
    <div className="pwa-rate-tile">
      <div className="pwa-rate-head">
        <strong>{props.title}</strong>
        <span>实时</span>
      </div>
      <div className="pwa-rate-lines">
        <div>
          <span>{props.topLabel}</span>
          <strong>{props.topValue}</strong>
        </div>
        <div>
          <span>{props.bottomLabel}</span>
          <strong>{props.bottomValue}</strong>
        </div>
      </div>
    </div>
  );
}

function HistoryStrip(props: {
  points: Array<{ ts: number; value: number }>;
}) {
  if (props.points.length < 2) {
    return <div className="pwa-history-empty">等待更多样本</div>;
  }

  const sampledPoints = samplePoints(props.points, 48);
  const width = 520;
  const height = 96;
  const padX = 8;
  const padY = 8;
  const minTs = sampledPoints[0].ts;
  const maxTs = sampledPoints[sampledPoints.length - 1].ts;
  const rangeTs = Math.max(1, maxTs - minTs);
  const maxValue = Math.max(100, ...sampledPoints.map((item) => item.value));
  const plotWidth = width - padX * 2;
  const plotHeight = height - padY * 2;

  const coordinates = sampledPoints.map((point) => {
    const x = padX + ((point.ts - minTs) / rangeTs) * plotWidth;
    const y = padY + (1 - point.value / maxValue) * plotHeight;
    return { x, y };
  });
  const linePoints = coordinates
    .map((point) => `${point.x.toFixed(2)},${point.y.toFixed(2)}`)
    .join(" ");
  const areaPoints = [
    `${padX},${height - padY}`,
    ...coordinates.map((point) => `${point.x.toFixed(2)},${point.y.toFixed(2)}`),
    `${width - padX},${height - padY}`,
  ].join(" ");
  const thresholdY = padY + (1 - 80 / maxValue) * plotHeight;
  const chartTone = getThresholdTone(Math.max(...sampledPoints.map((item) => item.value)));

  return (
    <div className={`pwa-history-chart tone-${chartTone}`} aria-hidden="true">
      <svg viewBox={`0 0 ${width} ${height}`} preserveAspectRatio="none">
        <line
          className="pwa-history-threshold"
          x1={padX}
          y1={thresholdY}
          x2={width - padX}
          y2={thresholdY}
        />
        <polygon className="pwa-history-area" points={areaPoints} />
        <polyline className="pwa-history-line" points={linePoints} />
      </svg>
    </div>
  );
}

function InsetTrend(props: {
  label: string;
  metricKey: MetricKey;
  primary: Array<{ ts: number; value: number }>;
}) {
  return (
    <div className="pwa-inset-trend">
      <span>{props.label}</span>
      <MiniSparkline metricKey={props.metricKey} points={props.primary} />
    </div>
  );
}

function DualInsetTrend(props: {
  label: string;
  topLabel: string;
  bottomLabel: string;
  topMetricKey: MetricKey;
  bottomMetricKey: MetricKey;
  topPoints: Array<{ ts: number; value: number }>;
  bottomPoints: Array<{ ts: number; value: number }>;
}) {
  return (
    <div className="pwa-inset-trend dual">
      <span>{props.label}</span>
      <DualSparkline
        bottomLabel={props.bottomLabel}
        bottomMetricKey={props.bottomMetricKey}
        bottomPoints={props.bottomPoints}
        topLabel={props.topLabel}
        topMetricKey={props.topMetricKey}
        topPoints={props.topPoints}
      />
    </div>
  );
}

function MiniSparkline(props: {
  points: Array<{ ts: number; value: number }>;
  metricKey?: MetricKey;
}) {
  if (props.points.length < 2) {
    return <div className="pwa-sparkline-empty">等待更多样本</div>;
  }

  const sampledPoints = samplePoints(props.points, 32);
  const width = 122;
  const height = 44;
  const padX = 3;
  const padY = 4;
  const minTs = sampledPoints[0].ts;
  const maxTs = sampledPoints[sampledPoints.length - 1].ts;
  const maxValue = Math.max(1, ...sampledPoints.map((item) => item.value)) * 1.1;
  const rangeTs = Math.max(1, maxTs - minTs);

  const linePoints = sampledPoints
    .map((point) => {
      const x = padX + ((point.ts - minTs) / rangeTs) * (width - padX * 2);
      const y = padY + (1 - point.value / maxValue) * (height - padY * 2);
      return `${x.toFixed(2)},${y.toFixed(2)}`;
    })
    .join(" ");

  return (
    <div className={`pwa-sparkline ${props.metricKey ? `metric-${props.metricKey}` : ""}`}>
      <svg viewBox={`0 0 ${width} ${height}`} preserveAspectRatio="none" aria-label="Metric trend">
        <polyline className="pwa-sparkline-line" points={linePoints} />
      </svg>
    </div>
  );
}

function DualSparkline(props: {
  topLabel: string;
  bottomLabel: string;
  topMetricKey: MetricKey;
  bottomMetricKey: MetricKey;
  topPoints: Array<{ ts: number; value: number }>;
  bottomPoints: Array<{ ts: number; value: number }>;
}) {
  const topPoints = samplePoints(props.topPoints, 48);
  const bottomPoints = samplePoints(props.bottomPoints, 48);
  const allPoints = [...topPoints, ...bottomPoints];

  if (topPoints.length < 2 || bottomPoints.length < 2 || !allPoints.length) {
    return <div className="pwa-sparkline-empty">等待更多样本</div>;
  }

  const width = 220;
  const height = 88;
  const padX = 4;
  const padY = 6;
  const minTs = Math.min(...allPoints.map((point) => point.ts));
  const maxTs = Math.max(...allPoints.map((point) => point.ts));
  const rangeTs = Math.max(1, maxTs - minTs);
  const plotWidth = width - padX * 2;
  const plotHeight = height - padY * 2;
  const centerY = padY + plotHeight / 2;
  const halfHeight = plotHeight / 2;
  const topMax = Math.max(1, ...topPoints.map((point) => point.value)) * 1.05;
  const bottomMax = Math.max(1, ...bottomPoints.map((point) => point.value)) * 1.05;

  const makeLine = (
    points: Array<{ ts: number; value: number }>,
    maxValue: number,
    direction: "top" | "bottom",
  ) =>
    points
      .map((point) => {
        const x = padX + ((point.ts - minTs) / rangeTs) * plotWidth;
        const offset = (point.value / maxValue) * (halfHeight - 4);
        const y = direction === "top" ? centerY - offset : centerY + offset;
        return `${x.toFixed(2)},${y.toFixed(2)}`;
      })
      .join(" ");

  const makeArea = (
    points: Array<{ ts: number; value: number }>,
    maxValue: number,
    direction: "top" | "bottom",
  ) => {
    const line = points.map((point) => {
      const x = padX + ((point.ts - minTs) / rangeTs) * plotWidth;
      const offset = (point.value / maxValue) * (halfHeight - 4);
      const y = direction === "top" ? centerY - offset : centerY + offset;
      return `${x.toFixed(2)},${y.toFixed(2)}`;
    });
    return [
      `${padX},${centerY}`,
      ...line,
      `${width - padX},${centerY}`,
    ].join(" ");
  };

  return (
    <div className="pwa-sparkline dual">
      <svg viewBox={`0 0 ${width} ${height}`} preserveAspectRatio="none" aria-label={`${props.topLabel}/${props.bottomLabel} trend`}>
        <line className="pwa-sparkline-midline" x1={padX} y1={centerY} x2={width - padX} y2={centerY} />
        <polygon className={`pwa-sparkline-area metric-${props.topMetricKey}`} points={makeArea(topPoints, topMax, "top")} />
        <polygon className={`pwa-sparkline-area metric-${props.bottomMetricKey}`} points={makeArea(bottomPoints, bottomMax, "bottom")} />
        <polyline className={`pwa-sparkline-line metric-${props.topMetricKey}`} points={makeLine(topPoints, topMax, "top")} />
        <polyline className={`pwa-sparkline-line metric-${props.bottomMetricKey}`} points={makeLine(bottomPoints, bottomMax, "bottom")} />
      </svg>
      <div className="pwa-dual-legend">
        <span className={`metric-${props.topMetricKey}`}>{props.topLabel}</span>
        <span className={`metric-${props.bottomMetricKey}`}>{props.bottomLabel}</span>
      </div>
    </div>
  );
}
