import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type PropsWithChildren,
} from "react";
import { useAppConfig } from "../config/AppConfigProvider";
import { HttpError } from "../../lib/http";
import { writeAuthorizedJson } from "./api";
import { useQueryClient } from "@tanstack/react-query";

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
  refreshSession: () => Promise<boolean>;
  signOut: () => Promise<void>;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: PropsWithChildren) {
  const { config } = useAppConfig();
  const [initializing, setInitializing] = useState(true);
  const [user, setUser] = useState<AuthUser | null>(null);
  const revision = useRef(0);
  const identity = useRef("");
  const queryClient = useQueryClient();

  const applyIdentity = useCallback(async (next: AuthUser | null, current: number) => {
    const nextIdentity = next ? `${next.tenant_code}:${next.id}:${next.role}:${next.status}` : "";
    if (identity.current !== nextIdentity) {
      await queryClient.cancelQueries();
      if (current !== revision.current) return false;
      queryClient.clear();
      identity.current = nextIdentity;
    }
    if (current !== revision.current) return false;
    setUser(next);
    return Boolean(next);
  }, [queryClient]);

  const sessionUrl = `${config.apiBaseUrl}/auth/session`;

  const refreshSession = useCallback(async () => {
    const current = ++revision.current;
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
      if (current !== revision.current) return false;
      return await applyIdentity(payload.authenticated ? payload.user || null : null, current);
    } catch {
      if (current === revision.current) await applyIdentity(null, current);
      return false;
    } finally {
      if (current === revision.current) setInitializing(false);
    }
  }, [sessionUrl, applyIdentity]);

  useEffect(() => {
    void refreshSession();
    return () => { revision.current++; };
  }, [refreshSession]);

  const signOut = useCallback(async () => {
    const current = ++revision.current;
    try {
      await writeAuthorizedJson<void>(config.apiBaseUrl, "/auth/logout", {});
    } catch (error) {
      if (!(error instanceof HttpError && error.status === 401)) throw error;
    }
    await applyIdentity(null, current);
  }, [config.apiBaseUrl, applyIdentity]);

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
