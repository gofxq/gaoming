import type { PropsWithChildren } from "react";
import { AppConfigProvider } from "./AppConfigProvider";
import { TenantProvider } from "./TenantProvider";

export function AppProviders({ children }: PropsWithChildren) {
  return (
    <AppConfigProvider>
      <TenantProvider>{children}</TenantProvider>
    </AppConfigProvider>
  );
}
