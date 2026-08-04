import type { PropsWithChildren } from "react";
import { QueryClientProvider } from "@tanstack/react-query";
import { queryClient } from "../../shared/lib/queryClient";
import { AppConfigProvider } from "../../shared/features/config/AppConfigProvider";
import { AppearanceProvider } from "../../shared/features/appearance/AppearanceProvider";
import { AuthProvider } from "../../shared/features/auth/AuthProvider";
import { TenantProvider } from "../../shared/features/tenant/TenantProvider";

export function AppProviders({ children }: PropsWithChildren) {
  return (
    <QueryClientProvider client={queryClient}>
      <AppearanceProvider>
        <AppConfigProvider>
          <TenantProvider>
            <AuthProvider>{children}</AuthProvider>
          </TenantProvider>
        </AppConfigProvider>
      </AppearanceProvider>
    </QueryClientProvider>
  );
}
