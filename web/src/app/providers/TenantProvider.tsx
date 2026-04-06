import {
  createContext,
  useContext,
  useMemo,
  useState,
  type PropsWithChildren,
} from "react";

type TenantContextValue = {
  tenantCode: string;
  setTenantCode: (tenantCode: string) => void;
};

const TenantContext = createContext<TenantContextValue | null>(null);

export function TenantProvider({ children }: PropsWithChildren) {
  const [tenantCode, setTenantCode] = useState("default");

  const value = useMemo<TenantContextValue>(
    () => ({
      tenantCode,
      setTenantCode,
    }),
    [tenantCode],
  );

  return <TenantContext.Provider value={value}>{children}</TenantContext.Provider>;
}

export function useTenant() {
  const context = useContext(TenantContext);
  if (!context) {
    throw new Error("useTenant must be used within TenantProvider");
  }
  return context;
}
