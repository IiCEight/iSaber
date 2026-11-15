"use client";

import { useEffect, useState, type ReactNode } from "react";
import { usePathname } from "next/navigation";

export default function ReadingSession({ owner, expiresAt, children }: { owner: boolean; expiresAt: string | null; children: ReactNode }) {
  const [hidden, setHidden] = useState(false);
  const pathname = usePathname();
  useEffect(() => {
    if (pathname === "/admin" || pathname.startsWith("/admin/")) return;
    let active = true;
    let reloading = false;
    let request: AbortController | null = null;
    let timer: ReturnType<typeof setTimeout> | undefined;
    function reload() { if (active && !reloading) { reloading = true; setHidden(true); window.location.reload(); } }
    async function check() {
      request?.abort();
      const controller = new AbortController(); request = controller;
      try {
        const response = await fetch("/api/v1/auth/session", { credentials: "same-origin", cache: "no-store", signal: AbortSignal.any([controller.signal, AbortSignal.timeout(5000)]) });
        if (!active || reloading || request !== controller) return;
        if (response.status === 401) { if (owner) reload(); else setHidden(false); }
        else if (response.ok) {
          const session = await response.json() as { expires_at?: string };
          if (!active || reloading || request !== controller || typeof session.expires_at !== "string" || !Number.isFinite(Date.parse(session.expires_at))) return;
          if (!owner) reload(); else setHidden(false);
        }
      } catch { /* A temporary outage does not change the authenticated scope. */ }
    }
    function visible() { if (document.visibilityState === "visible") void check(); }
    function restored(event: PageTransitionEvent) { if (event.persisted) { if (owner) setHidden(true); void check(); } }
    function deadline() {
      if (!expiresAt) return;
      const remaining = Date.parse(expiresAt) - Date.now();
      if (!Number.isFinite(remaining)) return;
      if (remaining <= 0) { void check(); timer = setTimeout(deadline, 60000); return; }
      timer = setTimeout(deadline, Math.min(remaining + 100, 2147483647));
    }
    const channel = typeof BroadcastChannel === "undefined" ? null : new BroadcastChannel("isaber:auth");
    if (channel) channel.onmessage = event => { if (event.data === "signed-out" && owner) reload(); else void check(); };
    window.addEventListener("focus", check);
    window.addEventListener("pageshow", restored);
    document.addEventListener("visibilitychange", visible);
    void check(); deadline();
    return () => { active = false; request?.abort(); clearTimeout(timer); channel?.close(); window.removeEventListener("focus", check); window.removeEventListener("pageshow", restored); document.removeEventListener("visibilitychange", visible); };
  }, [owner, expiresAt, pathname]);
  return hidden ? <p className="reading-session-status" role="status">Refreshing your reading view…</p> : children;
}
