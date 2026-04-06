import type { PropsWithChildren } from "react";
import { AppConfigProvider } from "./AppConfigProvider";
import { AuthProvider } from "./AuthProvider";
import { TenantProvider } from "./TenantProvider";

export function AppProviders({ children }: PropsWithChildren) {
  return (
    <AppConfigProvider>
      <AuthProvider>
        <TenantProvider>{children}</TenantProvider>
      </AuthProvider>
    </AppConfigProvider>
  );
}
