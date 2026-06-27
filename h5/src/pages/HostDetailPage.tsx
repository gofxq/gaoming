import {
  IonBackButton,
  IonButtons,
  IonCard,
  IonCardContent,
  IonContent,
  IonGrid,
  IonHeader,
  IonPage,
  IonProgressBar,
  IonRow,
  IonSegment,
  IonSegmentButton,
  IonText,
  IonTitle,
  IonToolbar,
} from "@ionic/react";
import { useMemo, useState } from "react";
import { useParams } from "react-router-dom";
import { useAppConfig } from "../app/providers/AppConfigProvider";
import { MetricCard } from "../components/MetricCard";
import { MetricSparkline } from "../components/MetricSparkline";
import { StatusBadge } from "../components/StatusBadge";
import {
  WINDOW_OPTIONS,
  currentWindowLabel,
  formatAgo,
  formatMetricValue,
  getWindowPoints,
  type MetricKey,
} from "../data/dashboard";
import { useLiveHostsData } from "../data/useLiveHostsData";

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

export function HostDetailPage() {
  const { tenantCode = "default", hostUID = "" } = useParams<RouteParams>();
  const config = useAppConfig();
  const [selectedWindowSec, setSelectedWindowSec] = useState(300);
  const { agents, histories, lastUpdated, streamState } = useLiveHostsData({
    apiBaseUrl: config.apiBaseUrl,
    streamPath: config.streamPath,
    tenantCode,
  });
  const host = agents[hostUID];
  const history = histories[hostUID];

  const metricSamples = useMemo(
    () =>
      DETAIL_METRICS.reduce(
        (sum, metric) => sum + getWindowPoints(history, metric.key, selectedWindowSec).length,
        0,
      ),
    [history, selectedWindowSec],
  );

  return (
    <IonPage>
      <IonHeader translucent>
        <IonToolbar>
          <IonButtons slot="start">
            <IonBackButton defaultHref={`/${tenantCode}`} text="返回" />
          </IonButtons>
          <IonTitle>{host ? host.hostname || host.host_uid : "主机详情"}</IonTitle>
        </IonToolbar>
      </IonHeader>
      <IonContent fullscreen>
        {!host ? (
          <IonCard className="detail-empty">
            <IonCardContent>当前设备不存在，或数据还未同步完成。</IonCardContent>
          </IonCard>
        ) : (
          <>
            <section className="detail-hero">
              <div>
                <IonText color="medium">
                  <span>{host.primary_ip || host.host_uid}</span>
                </IonText>
                <h1>{host.hostname || host.host_uid}</h1>
              </div>
              <StatusBadge stateCode={host.overall_state} />
            </section>

            <IonCard className="detail-meta-card">
              <IonCardContent>
                <div className="detail-meta-grid">
                  <span>
                    <IonText color="medium">心跳</IonText>
                    <strong>{formatAgo(host.last_agent_seen_at)}</strong>
                  </span>
                  <span>
                    <IonText color="medium">指标</IonText>
                    <strong>{formatAgo(host.last_metric_at)}</strong>
                  </span>
                  <span>
                    <IonText color="medium">版本</IonText>
                    <strong>v{host.version || 0}</strong>
                  </span>
                  <span>
                    <IonText color="medium">推送</IonText>
                    <strong>{streamState}</strong>
                  </span>
                </div>
              </IonCardContent>
            </IonCard>

            <IonSegment
              className="window-segment"
              value={String(selectedWindowSec)}
              onIonChange={(event) => setSelectedWindowSec(Number(event.detail.value || 300))}
            >
              {WINDOW_OPTIONS.map((option) => (
                <IonSegmentButton key={option.seconds} value={String(option.seconds)}>
                  {option.label}
                </IonSegmentButton>
              ))}
            </IonSegment>

            <p className="detail-window-note">
              {currentWindowLabel(selectedWindowSec)} 内累计 {metricSamples} 个样本，最近刷新{" "}
              {lastUpdated ? new Date(lastUpdated).toLocaleTimeString() : "--"}
            </p>

            <IonGrid className="detail-metric-grid">
              <IonRow>
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
              </IonRow>
            </IonGrid>

            <IonCard className="trend-card">
              <IonCardContent>
                <div className="trend-head">
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
