import { Button, Card, Progress, Space, Tag, Typography } from "@douyinfe/semi-ui";
import { useOutletContext } from "react-router-dom";
import type { ShellOutletContext } from "../components/layout/Shell";
import {
  currentWindowLabel,
  formatAgo,
  formatMetricValue,
  getWindowPoints,
  stateLabel,
  type HostHistoryMap,
  type HostSnapshot,
  type MetricKey,
} from "../features/hosts/model";
import "./PcDashboardPage.css";

const { Text } = Typography;

const CHARTS: Array<{
  title: string;
  metricKey: MetricKey;
  color: string;
}> = [
  { title: "CPU", metricKey: "cpu_usage_pct", color: "#1677ff" },
  { title: "内存", metricKey: "mem_used_pct", color: "#00a870" },
  { title: "磁盘", metricKey: "disk_used_pct", color: "#f59e0b" },
  { title: "网络 RX", metricKey: "net_rx_bps", color: "#8b5cf6" },
  { title: "网络 TX", metricKey: "net_tx_bps", color: "#ec4899" },
  { title: "磁盘读", metricKey: "disk_read_bps", color: "#0f766e" },
  { title: "磁盘写", metricKey: "disk_write_bps", color: "#b7791f" },
  { title: "负载", metricKey: "load1", color: "#475569" },
];

const DETAIL_METRICS: Array<{ label: string; metricKey: MetricKey }> = [
  { label: "CPU", metricKey: "cpu_usage_pct" },
  { label: "内存使用", metricKey: "mem_used_pct" },
  { label: "可用内存", metricKey: "mem_available_bytes" },
  { label: "Swap", metricKey: "swap_used_pct" },
  { label: "磁盘使用", metricKey: "disk_used_pct" },
  { label: "磁盘剩余", metricKey: "disk_free_bytes" },
  { label: "inode", metricKey: "disk_inodes_used_pct" },
  { label: "磁盘读", metricKey: "disk_read_bps" },
  { label: "磁盘写", metricKey: "disk_write_bps" },
  { label: "读 IOPS", metricKey: "disk_read_iops" },
  { label: "写 IOPS", metricKey: "disk_write_iops" },
  { label: "Load1", metricKey: "load1" },
  { label: "网络 RX", metricKey: "net_rx_bps" },
  { label: "网络 TX", metricKey: "net_tx_bps" },
  { label: "收包", metricKey: "net_rx_packets_ps" },
  { label: "发包", metricKey: "net_tx_packets_ps" },
];

function stateTagColor(state?: number) {
  switch (state) {
    case 1:
      return "green";
    case 2:
      return "orange";
    case 3:
    case 4:
      return "red";
    case 5:
      return "blue";
    default:
      return "grey";
  }
}

function metricNumber(host: HostSnapshot | undefined, key: MetricKey) {
  if (!host) {
    return 0;
  }
  return Number(host[key] || 0);
}

function latestPoint(
  history: HostHistoryMap | undefined,
  host: HostSnapshot | undefined,
  key: MetricKey,
  windowSec: number,
) {
  const points = getWindowPoints(history, key, windowSec);
  return points.length ? points[points.length - 1].value : metricNumber(host, key);
}

export function PcDashboardPage() {
  const { expandedHostUID, histories, lastUpdated, selectedWindowSec, setExpandedHostUID, sortedHosts } =
    useOutletContext<ShellOutletContext>();

  const expandedHost = sortedHosts.find((host) => host.host_uid === expandedHostUID);
  const expandedHistory = expandedHostUID ? histories[expandedHostUID] : undefined;
  const avgCPU = sortedHosts.length
    ? sortedHosts.reduce((sum, host) => sum + Number(host.cpu_usage_pct || 0), 0) / sortedHosts.length
    : 0;
  const avgMem = sortedHosts.length
    ? sortedHosts.reduce((sum, host) => sum + Number(host.mem_used_pct || 0), 0) / sortedHosts.length
    : 0;

  return (
    <div className="pc-page">
      <Card
        className="pc-card"
        title="主机"
        headerExtraContent={
          <Text type="tertiary">
            最近刷新 {lastUpdated ? new Date(lastUpdated).toLocaleTimeString() : "--"}
          </Text>
        }
      >
        {sortedHosts.length ? (
          <div className="pc-host-grid">
            {sortedHosts.map((host) => (
              <HostOverviewCard
                key={host.host_uid}
                host={host}
                active={host.host_uid === expandedHostUID}
                onClick={() => setExpandedHostUID((current) => (current === host.host_uid ? "" : host.host_uid))}
              />
            ))}
          </div>
        ) : (
          <div className="pc-empty">等待 Agent 上报</div>
        )}
      </Card>

      {expandedHost ? (
        <Card
          className="pc-card pc-detail-card"
          title={expandedHost.hostname || expandedHost.host_uid}
          headerExtraContent={
            <Space>
              <Tag color={stateTagColor(expandedHost.overall_state)} size="large">
                {stateLabel(expandedHost.overall_state)}
              </Tag>
              <Button size="small" onClick={() => setExpandedHostUID("")}>
                收起
              </Button>
            </Space>
          }
        >
          <div className="pc-detail-summary">
            <SummaryItem label="IP" value={expandedHost.primary_ip || "--"} />
            <SummaryItem label="Host UID" value={expandedHost.host_uid} />
            <SummaryItem label="Agent" value={stateLabel(expandedHost.agent_state)} />
            <SummaryItem label="CPU" value={formatMetricValue("cpu_usage_pct", metricNumber(expandedHost, "cpu_usage_pct"))} />
            <SummaryItem label="内存" value={formatMetricValue("mem_used_pct", metricNumber(expandedHost, "mem_used_pct"))} />
            <SummaryItem label="磁盘" value={formatMetricValue("disk_used_pct", metricNumber(expandedHost, "disk_used_pct"))} />
            <SummaryItem label="负载" value={metricNumber(expandedHost, "load1").toFixed(2)} />
            <SummaryItem label="心跳" value={formatAgo(expandedHost.last_agent_seen_at)} />
            <SummaryItem label="指标时间" value={formatAgo(expandedHost.last_metric_at)} />
            <SummaryItem label="版本" value={String(expandedHost.version || 0)} />
          </div>

          <div className="pc-section-title">指标明细</div>
          <div className="pc-detail-metric-grid">
            {DETAIL_METRICS.map((metric) => (
              <DetailMetric
                key={metric.metricKey}
                label={metric.label}
                metricKey={metric.metricKey}
                value={metricNumber(expandedHost, metric.metricKey)}
              />
            ))}
          </div>

          <div className="pc-section-title">趋势</div>
          <div className="pc-chart-grid">
            {CHARTS.map((chart) => (
              <MetricChart
                key={chart.metricKey}
                title={chart.title}
                metricKey={chart.metricKey}
                color={chart.color}
                value={latestPoint(expandedHistory, expandedHost, chart.metricKey, selectedWindowSec)}
                points={getWindowPoints(expandedHistory, chart.metricKey, selectedWindowSec)}
                windowLabel={currentWindowLabel(selectedWindowSec)}
              />
            ))}
          </div>
        </Card>
      ) : null}

      <Card className="pc-card" title="集群资源">
        <div className="pc-resource-grid">
          <MetricDial label="平均 CPU" value={avgCPU} />
          <MetricDial label="平均内存" value={avgMem} />
        </div>
      </Card>
    </div>
  );
}

function HostOverviewCard(props: {
  host: HostSnapshot;
  active: boolean;
  onClick: () => void;
}) {
  const { host } = props;
  const stateClassName = `state-${stateTagColor(host.overall_state)}`;
  return (
    <button
      type="button"
      className={`pc-host-overview ${stateClassName} ${props.active ? "active" : ""}`}
      onClick={props.onClick}
    >
      <div className="pc-host-overview-head">
        <div className="pc-host-title">
          <strong>{host.hostname || host.host_uid}</strong>
          <span>{host.primary_ip || host.host_uid}</span>
        </div>
        <span className="pc-status-chip">
          <i />
          {stateLabel(host.overall_state)}
        </span>
      </div>

      <div className="pc-host-metrics">
        <MetricMini label="CPU" metricKey="cpu_usage_pct" value={Number(host.cpu_usage_pct || 0)} />
        <MetricMini label="内存" metricKey="mem_used_pct" value={Number(host.mem_used_pct || 0)} />
        <MetricMini label="磁盘" metricKey="disk_used_pct" value={Number(host.disk_used_pct || 0)} />
        <MetricMini label="RX" metricKey="net_rx_bps" value={Number(host.net_rx_bps || 0)} />
        <MetricMini label="TX" metricKey="net_tx_bps" value={Number(host.net_tx_bps || 0)} />
        <MetricMini label="读" metricKey="disk_read_bps" value={Number(host.disk_read_bps || 0)} />
        <MetricMini label="写" metricKey="disk_write_bps" value={Number(host.disk_write_bps || 0)} />
      </div>

      <div className="pc-host-overview-foot">
        <span>Load {Number(host.load1 || 0).toFixed(2)}</span>
        <span>Metric {formatAgo(host.last_metric_at)}</span>
        <span>Beat {formatAgo(host.last_agent_seen_at)}</span>
      </div>
    </button>
  );
}

function MetricMini(props: { label: string; metricKey: MetricKey; value: number }) {
  const percentMetric = props.metricKey.endsWith("_pct");
  return (
    <div className={`pc-mini-metric ${percentMetric ? "percent" : "rate"}`}>
      <div>
        <span>{props.label}</span>
        <strong>{formatMetricValue(props.metricKey, props.value)}</strong>
      </div>
      {percentMetric ? (
        <Progress percent={Math.min(100, Math.max(0, props.value))} showInfo={false} size="small" />
      ) : null}
    </div>
  );
}

function MetricDial(props: { label: string; value: number }) {
  return (
    <div className="pc-dial">
      <Progress type="circle" percent={Math.min(100, Math.max(0, props.value))} width={76} strokeWidth={7} />
      <Text strong>{props.label}</Text>
    </div>
  );
}

function SummaryItem(props: { label: string; value: string }) {
  return (
    <div className="pc-summary-item">
      <Text type="tertiary">{props.label}</Text>
      <strong>{props.value}</strong>
    </div>
  );
}

function DetailMetric(props: { label: string; metricKey: MetricKey; value: number }) {
  const percentMetric = props.metricKey.endsWith("_pct");
  return (
    <div className="pc-detail-metric">
      <div className="pc-detail-metric-head">
        <Text type="tertiary">{props.label}</Text>
        <strong>{formatMetricValue(props.metricKey, props.value)}</strong>
      </div>
      {percentMetric ? (
        <Progress percent={Math.min(100, Math.max(0, props.value))} showInfo={false} size="small" />
      ) : null}
    </div>
  );
}

function MetricChart(props: {
  title: string;
  metricKey: MetricKey;
  color: string;
  value: number;
  points: Array<{ ts: number; value: number }>;
  windowLabel: string;
}) {
  const width = 320;
  const height = 82;
  const padX = 10;
  const padY = 10;
  const plotWidth = width - padX * 2;
  const plotHeight = height - padY * 2;
  const drawable = props.points.length >= 2;
  const minTs = drawable ? Math.min(...props.points.map((point) => point.ts)) : 0;
  const maxTs = drawable ? Math.max(...props.points.map((point) => point.ts)) : 1;
  const maxValue = drawable ? Math.max(1, ...props.points.map((point) => point.value)) * 1.15 : 1;
  const rangeTs = Math.max(1, maxTs - minTs);
  const linePoints = drawable
    ? props.points
        .map((point) => {
          const x = padX + ((point.ts - minTs) / rangeTs) * plotWidth;
          const y = padY + (1 - point.value / maxValue) * plotHeight;
          return `${x.toFixed(2)},${y.toFixed(2)}`;
        })
        .join(" ")
    : "";

  return (
    <div className="pc-chart">
      <div className="pc-chart-head">
        <div>
          <Text type="tertiary">{props.title}</Text>
          <strong>{formatMetricValue(props.metricKey, props.value)}</strong>
        </div>
        <Tag>{props.windowLabel}</Tag>
      </div>
      <div className="pc-chart-frame">
        {drawable ? (
          <svg viewBox={`0 0 ${width} ${height}`} preserveAspectRatio="none" aria-label={`${props.title} trend`}>
            <polyline className="pc-chart-line" points={linePoints} style={{ stroke: props.color }} />
          </svg>
        ) : (
          <span>暂无样本</span>
        )}
      </div>
    </div>
  );
}
