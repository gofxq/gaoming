import {
  createContext,
  useContext,
  useMemo,
  useState,
  type PropsWithChildren,
} from "react";

type AuthContextValue = {
  authenticated: boolean;
  userName: string;
  signIn: (userName: string) => void;
  signOut: () => void;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: PropsWithChildren) {
  const [userName, setUserName] = useState("");

  const value = useMemo<AuthContextValue>(
    () => ({
      authenticated: userName.length > 0,
      userName,
      signIn: (nextUserName: string) => setUserName(nextUserName),
      signOut: () => setUserName(""),
    }),
    [userName],
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
