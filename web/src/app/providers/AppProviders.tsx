import type { PropsWithChildren } from "react";
import { QueryClientProvider } from "@tanstack/react-query";
import { queryClient } from "../../lib/queryClient";
import { AppConfigProvider } from "./AppConfigProvider";
import { AuthProvider } from "./AuthProvider";
import { TenantProvider } from "./TenantProvider";

export function AppProviders({ children }: PropsWithChildren) {
  return (
    <QueryClientProvider client={queryClient}>
      <AppConfigProvider>
        <TenantProvider>
          <AuthProvider>{children}</AuthProvider>
        </TenantProvider>
      </AppConfigProvider>
    </QueryClientProvider>
  );
}
