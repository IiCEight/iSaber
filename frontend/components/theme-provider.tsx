"use client";

import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react";

export type Theme = "light" | "dark";
export const themeCookie = "isaber_theme";
const ThemeContext = createContext<{ theme: Theme; toggle: () => void } | null>(null);

export default function ThemeProvider({ initialTheme, children }: { initialTheme: Theme; children: ReactNode }) {
  const [theme, setTheme] = useState(initialTheme);
  const apply = useCallback((next: Theme) => {
    document.documentElement.dataset.theme = next;
    setTheme(next);
  }, []);
  useEffect(() => {
    // A restored page or another tab may have an older React tree than the
    // saved preference. Only read our presentation cookie, never session data.
    const sync = () => {
      try {
        const saved = document.cookie.split(";").map(value => value.trim()).find(value => value.startsWith(`${themeCookie}=`))?.slice(themeCookie.length + 1);
        // With cookies blocked, keep the user's current in-memory selection.
        if (saved === "dark" || saved === "light") apply(saved);
      } catch { /* The server's default or current page selection remains usable. */ }
    };
    const visible = () => { if (!document.hidden) sync(); };
    window.addEventListener("pageshow", sync);
    window.addEventListener("focus", sync);
    document.addEventListener("visibilitychange", visible);
    return () => {
      window.removeEventListener("pageshow", sync);
      window.removeEventListener("focus", sync);
      document.removeEventListener("visibilitychange", visible);
    };
  }, [apply]);
  const toggle = () => {
    const next = theme === "light" ? "dark" : "light";
    apply(next);
    try {
      // biome-ignore lint/suspicious/noDocumentCookie: A fixed, allowlisted theme value needs a broadly supported cookie readable by SSR; it contains no account data.
      document.cookie = `${themeCookie}=${next}; Path=/; Max-Age=31536000; SameSite=Lax${location.protocol === "https:" ? "; Secure" : ""}`;
    } catch { /* The current page still switches when persistent storage is blocked. */ }
  };
  return <ThemeContext.Provider value={{ theme, toggle }}>{children}</ThemeContext.Provider>;
}

export function ThemeToggle() {
  const context = useContext(ThemeContext);
  const [ready, setReady] = useState(false);
  useEffect(() => setReady(true), []);
  if (!context) return null;
  const dark = context.theme === "dark";
  return <button className="theme-toggle" type="button" disabled={!ready} onClick={context.toggle} aria-label={dark ? "Switch to light theme" : "Switch to dark theme"} title={dark ? "Switch to light theme" : "Switch to dark theme"}>
    <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      {dark ? <><circle cx="12" cy="12" r="4" /><path d="M12 2v2m0 16v2M2 12h2m16 0h2M5 5l1.5 1.5m11 11L19 19M5 19l1.5-1.5m11-11L19 5" /></> : <path d="M20.5 14A9 9 0 0 1 10 3.5 9 9 0 1 0 20.5 14Z" />}
    </svg><span>{dark ? "Light" : "Dark"}</span>
  </button>;
}
