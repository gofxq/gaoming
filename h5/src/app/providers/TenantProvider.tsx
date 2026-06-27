import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  type PropsWithChildren,
} from "react";

type TenantContextValue = {
  tenantCode: string;
  setTenantCode: (tenantCode: string) => void;
};

const TenantContext = createContext<TenantContextValue | null>(null);
const DEFAULT_TENANT_CODE = "default";

function normalizeTenantCode(value: string | null | undefined) {
  const next = value?.trim();
  return next || DEFAULT_TENANT_CODE;
}

function readTenantCodeFromLocation() {
  if (typeof window === "undefined") {
    return DEFAULT_TENANT_CODE;
  }

  const [tenantCode] = window.location.pathname.split("/").filter(Boolean);
  return normalizeTenantCode(tenantCode);
}

export function TenantProvider({ children }: PropsWithChildren) {
  const [tenantCode, setTenantCodeState] = useState(readTenantCodeFromLocation);

  useEffect(() => {
    const handleLocationChange = () => {
      setTenantCodeState(readTenantCodeFromLocation());
    };

    window.addEventListener("popstate", handleLocationChange);
    return () => window.removeEventListener("popstate", handleLocationChange);
  }, []);

  const value = useMemo<TenantContextValue>(
    () => ({
      tenantCode,
      setTenantCode: (nextTenantCode) => setTenantCodeState(normalizeTenantCode(nextTenantCode)),
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
