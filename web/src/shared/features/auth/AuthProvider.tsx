import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type PropsWithChildren,
} from "react";
import { useAppConfig } from "../config/AppConfigProvider";

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
  user: AuthUser | null;
  refreshSession: () => Promise<void>;
  signOut: () => Promise<void>;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: PropsWithChildren) {
  const { config } = useAppConfig();
  const [initializing, setInitializing] = useState(true);
  const [user, setUser] = useState<AuthUser | null>(null);

  const sessionUrl = `${config.apiBaseUrl}/auth/session`;
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
        user?: AuthUser;
      };
      setUser(payload.authenticated ? payload.user || null : null);
    } finally {
      setInitializing(false);
    }
  }, [sessionUrl]);

  useEffect(() => {
    void refreshSession();
  }, [refreshSession]);

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
      user,
      refreshSession,
      signOut,
    }),
    [initializing, refreshSession, signOut, user],
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
