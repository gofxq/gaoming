import {
  IonCard,
  IonCardContent,
  IonChip,
  IonContent,
  IonHeader,
  IonLabel,
  IonList,
  IonPage,
  IonRefresher,
  IonRefresherContent,
  IonText,
  IonTitle,
  IonToolbar,
  type RefresherEventDetail,
} from "@ionic/react";
import { useParams } from "react-router-dom";
import { useAppConfig } from "../app/providers/AppConfigProvider";
import { HostCard } from "../components/HostCard";
import {
  formatMetricValue,
  getWindowPoints,
} from "../data/dashboard";
import { useLiveHostsData } from "../data/useLiveHostsData";

type RouteParams = {
  tenantCode?: string;
};

export function HostListPage() {
  const { tenantCode = "default" } = useParams<RouteParams>();
  const config = useAppConfig();
  const {
    histories,
    lastUpdated,
    reloadHosts,
    sortedHosts,
    streamEventCount,
    streamState,
  } = useLiveHostsData({
    apiBaseUrl: config.apiBaseUrl,
    streamPath: config.streamPath,
    tenantCode,
  });

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
    <IonPage>
      <IonHeader translucent>
        <IonToolbar>
          <IonTitle>Gaoming</IonTitle>
        </IonToolbar>
      </IonHeader>
      <IonContent fullscreen>
        <IonRefresher slot="fixed" onIonRefresh={handleRefresh}>
          <IonRefresherContent />
        </IonRefresher>

        <section className="page-hero">
          <IonText color="medium">
            <span>Tenant · {tenantCode}</span>
          </IonText>
          <h1>主机状态</h1>
          <div className="hero-chips">
            <IonChip color={streamState === "实时推送中" ? "success" : "warning"}>
              <IonLabel>{streamState}</IonLabel>
            </IonChip>
            <IonChip>
              <IonLabel>事件 {streamEventCount}</IonLabel>
            </IonChip>
          </div>
        </section>

        <section className="summary-grid">
          <IonCard>
            <IonCardContent>
              <span>在线</span>
              <strong>{onlineCount}/{sortedHosts.length || 0}</strong>
            </IonCardContent>
          </IonCard>
          <IonCard>
            <IonCardContent>
              <span>平均 CPU</span>
              <strong>{formatMetricValue("cpu_usage_pct", avgCPU)}</strong>
            </IonCardContent>
          </IonCard>
          <IonCard>
            <IonCardContent>
              <span>平均内存</span>
              <strong>{formatMetricValue("mem_used_pct", avgMem)}</strong>
            </IonCardContent>
          </IonCard>
        </section>

        <IonList className="host-list" lines="none">
          {!sortedHosts.length ? (
            <IonCard>
              <IonCardContent className="empty-state">
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

        <p className="page-footnote">
          最近刷新 {lastUpdated ? new Date(lastUpdated).toLocaleTimeString() : "--"}
        </p>
      </IonContent>
    </IonPage>
  );
}
