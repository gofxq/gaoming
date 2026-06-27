import { IonApp } from "@ionic/react";
import { AppProviders } from "./providers/AppProviders";
import { AppRouter } from "./router";

export function App() {
  return (
    <IonApp>
      <AppProviders>
        <AppRouter />
      </AppProviders>
    </IonApp>
  );
}
