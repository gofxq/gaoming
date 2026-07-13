import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  type PropsWithChildren,
} from "react";

export type AppearanceSkin = "modern" | "pixel";
export type AppearanceTheme = "light" | "dark";

type AppearanceContextValue = {
  skin: AppearanceSkin;
  theme: AppearanceTheme;
  setSkin: (skin: AppearanceSkin) => void;
  setTheme: (theme: AppearanceTheme) => void;
  toggleSkin: () => void;
  toggleTheme: () => void;
};

const SKIN_STORAGE_KEY = "gaoming-skin";
const THEME_STORAGE_KEY = "gaoming-theme";
const AppearanceContext = createContext<AppearanceContextValue | null>(null);

function initialSkin(): AppearanceSkin {
  return document.documentElement.dataset.skin === "modern" ? "modern" : "pixel";
}

function initialTheme(): AppearanceTheme {
  return document.documentElement.dataset.theme === "dark" ? "dark" : "light";
}

export function AppearanceProvider({ children }: PropsWithChildren) {
  const [skin, setSkin] = useState<AppearanceSkin>(initialSkin);
  const [theme, setTheme] = useState<AppearanceTheme>(initialTheme);

  useEffect(() => {
    document.documentElement.dataset.skin = skin;
    window.localStorage.setItem(SKIN_STORAGE_KEY, skin);
  }, [skin]);

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    document.body.toggleAttribute("theme-mode", theme === "dark");
    window.localStorage.setItem(THEME_STORAGE_KEY, theme);
  }, [theme]);

  const value = useMemo<AppearanceContextValue>(
    () => ({
      skin,
      theme,
      setSkin,
      setTheme,
      toggleSkin: () => setSkin((current) => (current === "modern" ? "pixel" : "modern")),
      toggleTheme: () => setTheme((current) => (current === "light" ? "dark" : "light")),
    }),
    [skin, theme],
  );

  return <AppearanceContext.Provider value={value}>{children}</AppearanceContext.Provider>;
}

export function useAppearance() {
  const context = useContext(AppearanceContext);
  if (!context) {
    throw new Error("useAppearance must be used within AppearanceProvider");
  }
  return context;
}
