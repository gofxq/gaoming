import { IonCard, IonCardContent, IonText } from "@ionic/react";

export function MetricCard(props: {
  label: string;
  value: string;
  detail?: string;
  progress?: number;
}) {
  const progress = Math.max(0, Math.min(100, props.progress ?? 0));

  return (
    <IonCard className="metric-card">
      <IonCardContent>
        <IonText color="medium">
          <span className="metric-label">{props.label}</span>
        </IonText>
        <strong>{props.value}</strong>
        {props.detail ? <small>{props.detail}</small> : null}
        {props.progress !== undefined ? (
          <div className="metric-bar" aria-hidden="true">
            <span style={{ width: `${progress}%` }} />
          </div>
        ) : null}
      </IonCardContent>
    </IonCard>
  );
}
