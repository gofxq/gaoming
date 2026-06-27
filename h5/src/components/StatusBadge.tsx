import { IonBadge } from "@ionic/react";
import { stateClass, stateLabel } from "../data/dashboard";

export function StatusBadge(props: { stateCode?: number }) {
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
