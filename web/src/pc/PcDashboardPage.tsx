import { Button, Progress, Tag, Tooltip, Typography } from "@douyinfe/semi-ui";
import { IconClose, IconPulse, IconSearch, IconServer } from "@douyinfe/semi-icons";
import { useMemo, useState } from "react";
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

const CHARTS: Array<{ title: string; metricKey: MetricKey; color: string }> = [
  { title: "CPU", metricKey: "cpu_usage_pct", color: "#2563eb" },
  { title: "内存", metricKey: "mem_used_pct", color: "#0f9f6e" },
  { title: "磁盘", metricKey: "disk_used_pct", color: "#d97706" },
  { title: "网络 RX", metricKey: "net_rx_bps", color: "#7c3aed" },
  { title: "网络 TX", metricKey: "net_tx_bps", color: "#db2777" },
  { title: "磁盘读", metricKey: "disk_read_bps", color: "#0891b2" },
  { title: "磁盘写", metricKey: "disk_write_bps", color: "#ea580c" },
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

function stateTone(state?: number) {
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
  return host ? Number(host[key] || 0) : 0;
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
  const [query, setQuery] = useState("");

  const expandedHost = sortedHosts.find((host) => host.host_uid === expandedHostUID);
  const expandedHistory = expandedHostUID ? histories[expandedHostUID] : undefined;
  const onlineCount = sortedHosts.filter((host) => host.overall_state === 1).length;
  const issueCount = sortedHosts.filter((host) => host.overall_state === 2 || host.overall_state === 3).length;
  const offlineCount = sortedHosts.filter((host) => host.overall_state === 4).length;
  const avgCPU = averageMetric(sortedHosts, "cpu_usage_pct");
  const avgMem = averageMetric(sortedHosts, "mem_used_pct");
  const filteredHosts = useMemo(() => {
    const keyword = query.trim().toLocaleLowerCase();
    if (!keyword) return sortedHosts;
    return sortedHosts.filter((host) =>
      [host.hostname, host.primary_ip, host.host_uid].some((value) =>
        String(value || "").toLocaleLowerCase().includes(keyword),
      ),
    );
  }, [query, sortedHosts]);

  return (
    <div className="pc-page">
      <header className="page-heading">
        <div>
          <span className="section-kicker">INFRASTRUCTURE</span>
          <h1>运行总览</h1>
          <p>聚合所有节点的实时健康状态与资源负载。</p>
        </div>
        <div className="updated-at">
          <span>最近同步</span>
          <strong>{lastUpdated ? new Date(lastUpdated).toLocaleTimeString() : "等待数据"}</strong>
        </div>
      </header>

      <section className="cluster-summary glass-panel" aria-label="集群摘要">
        <SummaryStat label="全部主机" value={sortedHosts.length} tone="blue" />
        <SummaryStat label="运行正常" value={onlineCount} tone="green" />
        <SummaryStat label="需要关注" value={issueCount} tone="orange" />
        <SummaryStat label="已离线" value={offlineCount} tone="red" />
        <SummaryGauge label="平均 CPU" value={avgCPU} tone="blue" />
        <SummaryGauge label="平均内存" value={avgMem} tone="violet" />
      </section>

      <section className="pc-panel glass-panel">
        <div className="panel-toolbar">
          <div>
            <h2>主机列表</h2>
            <span>{filteredHosts.length} 台主机</span>
          </div>
          <label className="search-field">
            <IconSearch />
            <span className="sr-only">搜索主机</span>
            <input
              value={query}
              placeholder="搜索名称、IP 或 UID"
              onChange={(event) => setQuery(event.target.value)}
            />
            {query ? (
              <button type="button" aria-label="清空搜索" onClick={() => setQuery("")}>
                <IconClose />
              </button>
            ) : null}
          </label>
        </div>

        {filteredHosts.length ? (
          <div className="pc-host-grid">
            {filteredHosts.map((host) => (
              <HostOverviewCard
                key={host.host_uid}
                host={host}
                active={host.host_uid === expandedHostUID}
                onClick={() => setExpandedHostUID((current) => (current === host.host_uid ? "" : host.host_uid))}
              />
            ))}
          </div>
        ) : (
          <div className="pc-empty">
            <span><IconServer size="extra-large" /></span>
            <strong>{query ? "没有匹配的主机" : "等待 Agent 上报"}</strong>
            <p>{query ? "请调整搜索条件后重试。" : "主机首次上报后会自动出现在这里。"}</p>
          </div>
        )}
      </section>

      {expandedHost ? (
        <section className="pc-panel pc-detail-panel glass-panel" aria-label={`${expandedHost.hostname} 主机详情`}>
          <div className="detail-heading">
            <div className="detail-identity">
              <span className={`detail-icon state-${stateTone(expandedHost.overall_state)}`}><IconServer /></span>
              <div>
                <h2>{expandedHost.hostname || expandedHost.host_uid}</h2>
                <p>{expandedHost.primary_ip || expandedHost.host_uid}</p>
              </div>
              <span className={`status-badge state-${stateTone(expandedHost.overall_state)}`}>
                <i />{stateLabel(expandedHost.overall_state)}
              </span>
            </div>
            <Tooltip content="收起详情">
              <Button
                className="icon-button quiet"
                icon={<IconClose />}
                aria-label="收起主机详情"
                onClick={() => setExpandedHostUID("")}
              />
            </Tooltip>
          </div>

          <div className="pc-detail-summary">
            <SummaryItem label="Host UID" value={expandedHost.host_uid} />
            <SummaryItem label="Agent 状态" value={stateLabel(expandedHost.agent_state)} />
            <SummaryItem label="最近心跳" value={formatAgo(expandedHost.last_agent_seen_at)} />
            <SummaryItem label="指标时间" value={formatAgo(expandedHost.last_metric_at)} />
            <SummaryItem label="数据版本" value={String(expandedHost.version || 0)} />
          </div>

          <div className="detail-section-heading">
            <h3>指标明细</h3>
            <span>当前采样值</span>
          </div>
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

          <div className="detail-section-heading">
            <h3>性能趋势</h3>
            <span>时间范围 {currentWindowLabel(selectedWindowSec)}</span>
          </div>
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
        </section>
      ) : null}
    </div>
  );
}

function averageMetric(hosts: HostSnapshot[], key: MetricKey) {
  if (!hosts.length) return 0;
  return hosts.reduce((sum, host) => sum + metricNumber(host, key), 0) / hosts.length;
}

function SummaryStat(props: { label: string; value: number; tone: string }) {
  return (
    <div className={`summary-stat tone-${props.tone}`}>
      <span className="summary-icon"><IconServer /></span>
      <div><span>{props.label}</span><strong>{props.value}</strong></div>
    </div>
  );
}

function SummaryGauge(props: { label: string; value: number; tone: string }) {
  return (
    <div className={`summary-gauge tone-${props.tone}`}>
      <div><span>{props.label}</span><strong>{props.value.toFixed(1)}%</strong></div>
      <Progress percent={Math.min(100, Math.max(0, props.value))} showInfo={false} />
    </div>
  );
}

function HostOverviewCard(props: { host: HostSnapshot; active: boolean; onClick: () => void }) {
  const { host } = props;
  const tone = stateTone(host.overall_state);
  return (
    <button
      type="button"
      className={`pc-host-overview state-${tone} ${props.active ? "active" : ""}`}
      onClick={props.onClick}
    >
      <div className="pc-host-overview-head">
        <span className="host-icon"><IconServer /></span>
        <div className="pc-host-title">
          <strong>{host.hostname || host.host_uid}</strong>
          <span>{host.primary_ip || host.host_uid}</span>
        </div>
        <span className="pc-status-chip"><i />{stateLabel(host.overall_state)}</span>
      </div>

      <div className="pc-host-metrics">
        <MetricMini label="CPU" metricKey="cpu_usage_pct" value={Number(host.cpu_usage_pct || 0)} />
        <MetricMini label="内存" metricKey="mem_used_pct" value={Number(host.mem_used_pct || 0)} />
        <MetricMini label="磁盘" metricKey="disk_used_pct" value={Number(host.disk_used_pct || 0)} />
        <MetricMini label="网络 RX" metricKey="net_rx_bps" value={Number(host.net_rx_bps || 0)} />
      </div>

      <div className="pc-host-overview-foot">
        <span>LOAD <strong>{Number(host.load1 || 0).toFixed(2)}</strong></span>
        <span>METRIC <strong>{formatAgo(host.last_metric_at)}</strong></span>
        <span>HEARTBEAT <strong>{formatAgo(host.last_agent_seen_at)}</strong></span>
      </div>
    </button>
  );
}

function MetricMini(props: { label: string; metricKey: MetricKey; value: number }) {
  const percentMetric = props.metricKey.endsWith("_pct");
  return (
    <div className="pc-mini-metric">
      <div><span>{props.label}</span><strong>{formatMetricValue(props.metricKey, props.value)}</strong></div>
      {percentMetric ? (
        <Progress percent={Math.min(100, Math.max(0, props.value))} showInfo={false} size="small" />
      ) : <span className="metric-pulse"><i /><i /><i /><i /></span>}
    </div>
  );
}

function SummaryItem(props: { label: string; value: string }) {
  return (
    <div className="pc-summary-item">
      <Text type="tertiary">{props.label}</Text>
      <strong title={props.value}>{props.value}</strong>
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
  const height = 92;
  const padX = 8;
  const padY = 10;
  const plotWidth = width - padX * 2;
  const plotHeight = height - padY * 2;
  const drawable = props.points.length >= 2;
  const minTs = drawable ? Math.min(...props.points.map((point) => point.ts)) : 0;
  const maxTs = drawable ? Math.max(...props.points.map((point) => point.ts)) : 1;
  const maxValue = drawable ? Math.max(1, ...props.points.map((point) => point.value)) * 1.15 : 1;
  const rangeTs = Math.max(1, maxTs - minTs);
  const linePoints = drawable
    ? props.points.map((point) => {
        const x = padX + ((point.ts - minTs) / rangeTs) * plotWidth;
        const y = padY + (1 - point.value / maxValue) * plotHeight;
        return `${x.toFixed(2)},${y.toFixed(2)}`;
      }).join(" ")
    : "";
  const areaPoints = drawable ? `${padX},${height - padY} ${linePoints} ${width - padX},${height - padY}` : "";

  return (
    <div className="pc-chart">
      <div className="pc-chart-head">
        <div><Text type="tertiary">{props.title}</Text><strong>{formatMetricValue(props.metricKey, props.value)}</strong></div>
        <Tag>{props.windowLabel}</Tag>
      </div>
      <div className="pc-chart-frame">
        {drawable ? (
          <svg viewBox={`0 0 ${width} ${height}`} preserveAspectRatio="none" aria-label={`${props.title} trend`}>
            <line x1="0" y1="30" x2={width} y2="30" className="chart-grid-line" />
            <line x1="0" y1="61" x2={width} y2="61" className="chart-grid-line" />
            <polygon points={areaPoints} fill={props.color} opacity="0.08" />
            <polyline className="pc-chart-line" points={linePoints} style={{ stroke: props.color }} />
          </svg>
        ) : (
          <span><IconPulse /> 暂无样本</span>
        )}
      </div>
    </div>
  );
}
