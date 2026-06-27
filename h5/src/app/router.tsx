import { IonRouterOutlet } from "@ionic/react";
import { IonReactRouter } from "@ionic/react-router";
import { Redirect, Route, Switch, type RouteComponentProps } from "react-router-dom";
import { HostDetailPage } from "../pages/HostDetailPage";
import { HostListPage } from "../pages/HostListPage";

export function AppRouter() {
  return (
    <IonReactRouter>
      <IonRouterOutlet>
        <Switch>
          <Route exact path="/">
            <Redirect to="/default" />
          </Route>
          <Route
            exact
            path="/:tenantCode/pwa"
            render={({ match }: RouteComponentProps<{ tenantCode?: string }>) => (
              <Redirect to={`/${match.params.tenantCode || "default"}`} />
            )}
          />
          <Route
            exact
            path="/:tenantCode/pwa/:hostUID"
            render={({ match }: RouteComponentProps<{ tenantCode?: string; hostUID?: string }>) => {
              const params = match.params;
              return (
                <Redirect
                  to={`/${params.tenantCode || "default"}/hosts/${params.hostUID || ""}`}
                />
              );
            }}
          />
          <Route exact path="/:tenantCode/hosts/:hostUID" component={HostDetailPage} />
          <Route exact path="/:tenantCode" component={HostListPage} />
          <Route>
            <Redirect to="/default" />
          </Route>
        </Switch>
      </IonRouterOutlet>
    </IonReactRouter>
  );
}
