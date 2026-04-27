import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type PropsWithChildren,
} from "react";
import { useAppConfig } from "./AppConfigProvider";
import { useTenant } from "./TenantProvider";

type AuthUser = {
  id: number;
  tenant_code: string;
  display_name: string;
  avatar_url?: string;
  role: "admin" | "member";
  status: "active" | "disabled";
  last_login_at?: string;
  created_at: string;
  updated_at: string;
};

type AuthContextValue = {
  authenticated: boolean;
  initializing: boolean;
  weChatEnabled: boolean;
  user: AuthUser | null;
  refreshSession: () => Promise<void>;
  beginWeChatLogin: (returnTo?: string) => Promise<void>;
  signOut: () => Promise<void>;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: PropsWithChildren) {
  const { config } = useAppConfig();
  const { tenantCode } = useTenant();
  const [initializing, setInitializing] = useState(true);
  const [weChatEnabled, setWeChatEnabled] = useState(false);
  const [user, setUser] = useState<AuthUser | null>(null);

  const sessionUrl = `${config.apiBaseUrl}/auth/session`;
  const weChatUrlApi = `${config.apiBaseUrl}/auth/wechat/url`;
  const logoutUrl = `${config.apiBaseUrl}/auth/logout`;

  const refreshSession = useCallback(async () => {
    try {
      const response = await fetch(sessionUrl, {
        credentials: "include",
      });
      if (!response.ok) {
        throw new Error("load session failed");
      }

      const payload = (await response.json()) as {
        authenticated?: boolean;
        wechat_enabled?: boolean;
        user?: AuthUser;
      };
      setUser(payload.authenticated ? payload.user || null : null);
      setWeChatEnabled(Boolean(payload.wechat_enabled));
    } finally {
      setInitializing(false);
    }
  }, [sessionUrl]);

  useEffect(() => {
    void refreshSession();
  }, [refreshSession]);

  const beginWeChatLogin = useCallback(
    async (returnTo?: string) => {
      const nextReturnTo =
        returnTo ||
        `${window.location.pathname}${window.location.search}${window.location.hash}` ||
        `/${tenantCode}`;
      const url = new URL(weChatUrlApi, window.location.origin);
      url.searchParams.set("tenant", tenantCode);
      url.searchParams.set("return_to", nextReturnTo);

      const response = await fetch(url.toString(), {
        credentials: "include",
      });
      if (!response.ok) {
        const payload = (await response.json().catch(() => ({}))) as { error?: string };
        throw new Error(payload.error || "微信登录不可用");
      }

      const payload = (await response.json()) as { auth_url: string };
      window.location.href = payload.auth_url;
    },
    [tenantCode, weChatUrlApi],
  );

  const signOut = useCallback(async () => {
    await fetch(logoutUrl, {
      method: "POST",
      credentials: "include",
    });
    setUser(null);
  }, [logoutUrl]);

  const value = useMemo<AuthContextValue>(
    () => ({
      authenticated: Boolean(user),
      initializing,
      weChatEnabled,
      user,
      refreshSession,
      beginWeChatLogin,
      signOut,
    }),
    [beginWeChatLogin, initializing, refreshSession, signOut, user, weChatEnabled],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuth must be used within AuthProvider");
  }
  return context;
}
