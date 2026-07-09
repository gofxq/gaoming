import {
  IonApp,
  IonBadge,
  IonButton,
  IonButtons,
  IonCard,
  IonCardContent,
  IonChip,
  IonContent,
  IonHeader,
  IonItem,
  IonLabel,
  IonList,
  IonPage,
  IonProgressBar,
  IonRefresher,
  IonRefresherContent,
  IonSegment,
  IonSegmentButton,
  IonText,
  IonTitle,
  IonToolbar,
  type RefresherEventDetail,
} from "@ionic/react";
import { useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { useAppConfig } from "../../app/providers/AppConfigProvider";
import {
  WINDOW_OPTIONS,
  currentWindowLabel,
  formatAgo,
  formatMetricValue,
  getWindowPoints,
  stateClass,
  stateLabel,
  type HostSnapshot,
  type MetricKey,
} from "../dashboard/dashboard";
import { useLiveHostsData } from "../dashboard/useLiveHostsData";

type RouteParams = {
  tenantCode?: string;
  hostUID?: string;
};

const DETAIL_METRICS: Array<{ key: MetricKey; label: string }> = [
  { key: "cpu_usage_pct", label: "CPU" },
  { key: "mem_used_pct", label: "内存" },
  { key: "disk_used_pct", label: "磁盘" },
  { key: "load1", label: "负载" },
  { key: "net_rx_bps", label: "网络 RX" },
  { key: "net_tx_bps", label: "网络 TX" },
  { key: "disk_read_bps", label: "磁盘读" },
  { key: "disk_write_bps", label: "磁盘写" },
];

export function MobileAgentPage() {
  const { config } = useAppConfig();
  const { tenantCode = "default", hostUID } = useParams<RouteParams>();
  const [selectedWindowSec, setSelectedWindowSec] = useState(300);
  const data = useLiveHostsData({
    apiBaseUrl: config.apiBaseUrl,
    streamPath: config.streamPath,
    tenantCode,
  });
  const selectedHost = hostUID ? data.agents[hostUID] : undefined;

  return (
    <IonApp className="mobile-ion-app">
      {hostUID ? (
        <HostDetailView
          host={selectedHost}
          hostUID={hostUID}
          lastUpdated={data.lastUpdated}
          selectedWindowSec={selectedWindowSec}
          setSelectedWindowSec={setSelectedWindowSec}
          streamState={data.streamState}
          tenantCode={tenantCode}
          history={data.histories[hostUID]}
        />
      ) : (
        <HostListView
          histories={data.histories}
          lastUpdated={data.lastUpdated}
          reloadHosts={data.reloadHosts}
          sortedHosts={data.sortedHosts}
          streamEventCount={data.streamEventCount}
          streamState={data.streamState}
          tenantCode={tenantCode}
        />
      )}
    </IonApp>
  );
}

function HostListView(props: {
  histories: ReturnType<typeof useLiveHostsData>["histories"];
  lastUpdated: string;
  reloadHosts: () => Promise<void>;
  sortedHosts: HostSnapshot[];
  streamEventCount: number;
  streamState: string;
  tenantCode: string;
}) {
  const {
    histories,
    lastUpdated,
    reloadHosts,
    sortedHosts,
    streamEventCount,
    streamState,
    tenantCode,
  } = props;
  const onlineCount = sortedHosts.filter((host) => host.overall_state !== 4).length;
  const avgCPU = sortedHosts.length
    ? sortedHosts.reduce((sum, host) => sum + Number(host.cpu_usage_pct || 0), 0) / sortedHosts.length
    : 0;
  const avgMem = sortedHosts.length
    ? sortedHosts.reduce((sum, host) => sum + Number(host.mem_used_pct || 0), 0) / sortedHosts.length
    : 0;

  async function handleRefresh(event: CustomEvent<RefresherEventDetail>) {
    try {
      await reloadHosts();
    } finally {
      event.detail.complete();
    }
  }

  return (
    <IonPage className="mobile-ion-page">
      <IonHeader translucent>
        <IonToolbar>
          <IonTitle>Gaoming</IonTitle>
        </IonToolbar>
      </IonHeader>
      <IonContent fullscreen className="mobile-ion-content">
        <IonRefresher slot="fixed" onIonRefresh={handleRefresh}>
          <IonRefresherContent />
        </IonRefresher>

        <section className="mobile-ion-hero">
          <IonText color="medium">
            <span>Tenant · {tenantCode}</span>
          </IonText>
          <h1>主机状态</h1>
          <div className="mobile-ion-chips">
            <IonChip color={streamState === "实时推送中" ? "success" : "warning"}>
              <IonLabel>{streamState}</IonLabel>
            </IonChip>
            <IonChip>
              <IonLabel>事件 {streamEventCount}</IonLabel>
            </IonChip>
          </div>
        </section>

        <section className="mobile-ion-summary-grid">
          <SummaryCard label="在线" value={`${onlineCount}/${sortedHosts.length || 0}`} />
          <SummaryCard label="平均 CPU" value={formatMetricValue("cpu_usage_pct", avgCPU)} />
          <SummaryCard label="平均内存" value={formatMetricValue("mem_used_pct", avgMem)} />
        </section>

        <IonList className="mobile-ion-host-list" lines="none">
          {!sortedHosts.length ? (
            <IonCard className="mobile-ion-card">
              <IonCardContent className="mobile-ion-empty">
                等待 Agent 上报，或检查当前租户是否有主机数据。
              </IonCardContent>
            </IonCard>
          ) : (
            sortedHosts.map((host) => (
              <HostCard
                key={host.host_uid}
                cpuPoints={getWindowPoints(histories[host.host_uid], "cpu_usage_pct", 300)}
                host={host}
                tenantCode={tenantCode}
              />
            ))
          )}
        </IonList>

        <p className="mobile-ion-footnote">
          最近刷新 {lastUpdated ? new Date(lastUpdated).toLocaleTimeString() : "--"}
        </p>
      </IonContent>
    </IonPage>
  );
}

function HostDetailView(props: {
  history: ReturnType<typeof useLiveHostsData>["histories"][string] | undefined;
  host: HostSnapshot | undefined;
  hostUID: string;
  lastUpdated: string;
  selectedWindowSec: number;
  setSelectedWindowSec: (value: number) => void;
  streamState: string;
  tenantCode: string;
}) {
  const {
    history,
    host,
    hostUID,
    lastUpdated,
    selectedWindowSec,
    setSelectedWindowSec,
    streamState,
    tenantCode,
  } = props;
  const navigate = useNavigate();
  const metricSamples = useMemo(
    () =>
      DETAIL_METRICS.reduce(
        (sum, metric) => sum + getWindowPoints(history, metric.key, selectedWindowSec).length,
        0,
      ),
    [history, selectedWindowSec],
  );

  return (
    <IonPage className="mobile-ion-page">
      <IonHeader translucent>
        <IonToolbar>
          <IonButtons slot="start">
            <IonButton onClick={() => navigate(`/${tenantCode}/pwa`)}>返回</IonButton>
          </IonButtons>
          <IonTitle>{host ? host.hostname || host.host_uid : "主机详情"}</IonTitle>
        </IonToolbar>
      </IonHeader>
      <IonContent fullscreen className="mobile-ion-content">
        {!host ? (
          <IonCard className="mobile-ion-card mobile-ion-detail-empty">
            <IonCardContent>
              当前设备不存在，或数据还未同步完成。
              <br />
              <IonText color="medium">{hostUID}</IonText>
            </IonCardContent>
          </IonCard>
        ) : (
          <>
            <section className="mobile-ion-detail-hero">
              <div>
                <IonText color="medium">
                  <span>{host.primary_ip || host.host_uid}</span>
                </IonText>
                <h1>{host.hostname || host.host_uid}</h1>
              </div>
              <StatusBadge stateCode={host.overall_state} />
            </section>

            <IonCard className="mobile-ion-card mobile-ion-detail-meta-card">
              <IonCardContent>
                <div className="mobile-ion-detail-meta-grid">
                  <MetaCell label="心跳" value={formatAgo(host.last_agent_seen_at)} />
                  <MetaCell label="指标" value={formatAgo(host.last_metric_at)} />
                  <MetaCell label="版本" value={`v${host.version || 0}`} />
                  <MetaCell label="推送" value={streamState} />
                </div>
              </IonCardContent>
            </IonCard>

            <IonSegment
              className="mobile-ion-window-segment"
              value={String(selectedWindowSec)}
              onIonChange={(event) => setSelectedWindowSec(Number(event.detail.value || 300))}
            >
              {WINDOW_OPTIONS.map((option) => (
                <IonSegmentButton key={option.seconds} value={String(option.seconds)}>
                  {option.label}
                </IonSegmentButton>
              ))}
            </IonSegment>

            <p className="mobile-ion-footnote">
              {currentWindowLabel(selectedWindowSec)} 内累计 {metricSamples} 个样本，最近刷新{" "}
              {lastUpdated ? new Date(lastUpdated).toLocaleTimeString() : "--"}
            </p>

            <section className="mobile-ion-metric-grid">
              {DETAIL_METRICS.map((metric) => {
                const value = Number(host[metric.key] || 0);
                const points = getWindowPoints(history, metric.key, selectedWindowSec);
                return (
                  <MetricCard
                    key={metric.key}
                    detail={`${points.length} samples`}
                    label={metric.label}
                    progress={metric.key.endsWith("_pct") ? value : undefined}
                    value={formatMetricValue(metric.key, value)}
                  />
                );
              })}
            </section>

            <IonCard className="mobile-ion-card mobile-ion-trend-card">
              <IonCardContent>
                <div className="mobile-ion-trend-head">
                  <strong>CPU 趋势</strong>
                  <IonText color="medium">{currentWindowLabel(selectedWindowSec)}</IonText>
                </div>
                <MetricSparkline
                  metricKey="cpu_usage_pct"
                  points={getWindowPoints(history, "cpu_usage_pct", selectedWindowSec)}
                />
                <IonProgressBar value={Math.min(1, Number(host.cpu_usage_pct || 0) / 100)} />
              </IonCardContent>
            </IonCard>
          </>
        )}
      </IonContent>
    </IonPage>
  );
}

function SummaryCard(props: { label: string; value: string }) {
  return (
    <IonCard className="mobile-ion-card mobile-ion-summary-card">
      <IonCardContent>
        <span>{props.label}</span>
        <strong>{props.value}</strong>
      </IonCardContent>
    </IonCard>
  );
}

function HostCard(props: {
  cpuPoints: Array<{ ts: number; value: number }>;
  host: HostSnapshot;
  tenantCode: string;
}) {
  const { cpuPoints, host, tenantCode } = props;
  const navigate = useNavigate();
  const title = host.hostname || host.host_uid;

  return (
    <IonCard
      button
      className="mobile-ion-card mobile-ion-host-card"
      onClick={() => navigate(`/${tenantCode}/pwa/${encodeURIComponent(host.host_uid)}`)}
    >
      <IonItem lines="none">
        <IonLabel>
          <h2>{title}</h2>
          <p>{host.primary_ip || host.host_uid}</p>
        </IonLabel>
        <StatusBadge stateCode={host.overall_state} />
      </IonItem>
      <IonCardContent>
        <div className="mobile-ion-host-metric-grid">
          <MetaCell label="CPU" value={formatMetricValue("cpu_usage_pct", Number(host.cpu_usage_pct || 0))} />
          <MetaCell label="内存" value={formatMetricValue("mem_used_pct", Number(host.mem_used_pct || 0))} />
          <MetaCell label="磁盘" value={formatMetricValue("disk_used_pct", Number(host.disk_used_pct || 0))} />
          <MetaCell label="心跳" value={formatAgo(host.last_agent_seen_at)} />
        </div>
        <MetricSparkline metricKey="cpu_usage_pct" points={cpuPoints} />
        <span className="mobile-ion-card-link">查看详情</span>
      </IonCardContent>
    </IonCard>
  );
}

function MetricCard(props: {
  detail?: string;
  label: string;
  progress?: number;
  value: string;
}) {
  const progress = Math.max(0, Math.min(100, props.progress ?? 0));

  return (
    <IonCard className="mobile-ion-card mobile-ion-metric-card">
      <IonCardContent>
        <IonText color="medium">
          <span className="mobile-ion-metric-label">{props.label}</span>
        </IonText>
        <strong>{props.value}</strong>
        {props.detail ? <small>{props.detail}</small> : null}
        {props.progress !== undefined ? (
          <div className="mobile-ion-metric-bar" aria-hidden="true">
            <span style={{ width: `${progress}%` }} />
          </div>
        ) : null}
      </IonCardContent>
    </IonCard>
  );
}

function MetaCell(props: { label: string; value: string }) {
  return (
    <span>
      <IonText color="medium">{props.label}</IonText>
      <strong>{props.value}</strong>
    </span>
  );
}

function StatusBadge(props: { stateCode?: number }) {
  const tone = stateClass(props.stateCode);
  const color =
    tone === "up"
      ? "success"
      : tone === "warning"
        ? "warning"
        : tone === "critical" || tone === "offline"
          ? "danger"
          : "medium";

  return <IonBadge color={color}>{stateLabel(props.stateCode)}</IonBadge>;
}

function samplePoints(points: Array<{ ts: number; value: number }>, maxPoints: number) {
  if (points.length <= maxPoints) {
    return points;
  }
  const sampled: Array<{ ts: number; value: number }> = [];
  const lastIndex = points.length - 1;
  const step = lastIndex / (maxPoints - 1);
  for (let index = 0; index < maxPoints; index += 1) {
    sampled.push(points[Math.min(lastIndex, Math.round(index * step))]);
  }
  return sampled;
}

function MetricSparkline(props: {
  metricKey?: MetricKey;
  points: Array<{ ts: number; value: number }>;
}) {
  const points = samplePoints(props.points, 36);
  if (points.length < 2) {
    return <div className="mobile-ion-sparkline-empty">等待更多样本</div>;
  }

  const width = 220;
  const height = 72;
  const pad = 8;
  const values = points.map((point) => point.value);
  const min = Math.min(...values);
  const max = Math.max(...values);
  const range = max - min || 1;
  const lastIndex = points.length - 1;
  const linePoints = points
    .map((point, index) => {
      const x = pad + (index / lastIndex) * (width - pad * 2);
      const y = height - pad - ((point.value - min) / range) * (height - pad * 2);
      return `${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(" ");
  const areaPoints = `${pad},${height - pad} ${linePoints} ${width - pad},${height - pad}`;

  return (
    <svg
      className={`mobile-ion-sparkline ${props.metricKey ? `metric-${props.metricKey}` : ""}`}
      viewBox={`0 0 ${width} ${height}`}
      role="img"
      aria-label="metric trend"
    >
      <polygon className="mobile-ion-sparkline-area" points={areaPoints} />
      <polyline className="mobile-ion-sparkline-line" points={linePoints} />
    </svg>
  );
}
