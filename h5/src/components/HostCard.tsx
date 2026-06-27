import { IonCard, IonCardContent, IonItem, IonLabel, IonText } from "@ionic/react";
import {
  formatAgo,
  formatMetricValue,
  type HostSnapshot,
} from "../data/dashboard";
import { MetricSparkline } from "./MetricSparkline";
import { StatusBadge } from "./StatusBadge";

export function HostCard(props: {
  host: HostSnapshot;
  tenantCode: string;
  cpuPoints: Array<{ ts: number; value: number }>;
}) {
  const { host, tenantCode } = props;
  const title = host.hostname || host.host_uid;

  return (
    <IonCard className="host-card" routerLink={`/${tenantCode}/hosts/${host.host_uid}`}>
      <IonItem lines="none">
        <IonLabel>
          <h2>{title}</h2>
          <p>{host.primary_ip || host.host_uid}</p>
        </IonLabel>
        <StatusBadge stateCode={host.overall_state} />
      </IonItem>
      <IonCardContent>
        <div className="host-metric-grid">
          <span>
            <IonText color="medium">CPU</IonText>
            <strong>{formatMetricValue("cpu_usage_pct", Number(host.cpu_usage_pct || 0))}</strong>
          </span>
          <span>
            <IonText color="medium">内存</IonText>
            <strong>{formatMetricValue("mem_used_pct", Number(host.mem_used_pct || 0))}</strong>
          </span>
          <span>
            <IonText color="medium">磁盘</IonText>
            <strong>{formatMetricValue("disk_used_pct", Number(host.disk_used_pct || 0))}</strong>
          </span>
          <span>
            <IonText color="medium">心跳</IonText>
            <strong>{formatAgo(host.last_agent_seen_at)}</strong>
          </span>
        </div>
        <MetricSparkline metricKey="cpu_usage_pct" points={props.cpuPoints} />
        <span className="host-card-link">查看详情</span>
      </IonCardContent>
    </IonCard>
  );
}
