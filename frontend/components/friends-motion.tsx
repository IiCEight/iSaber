"use client";
import { createContext, useContext, useEffect, useRef, useState, type ReactNode } from "react";

const MotionPreference = createContext<{ reduced: boolean; setReduced: (value: boolean) => void } | null>(null);
export function FriendsMotionControl() {
  const preference = useContext(MotionPreference);
  return preference ? <label className="friends-motion-control"><input type="checkbox" checked={preference.reduced} onChange={event => preference.setReduced(event.target.checked)} />Reduce motion</label> : null;
}
export default function FriendsMotion({ children }: { children: ReactNode }) {
  const ref = useRef<HTMLDivElement>(null);
  const [override, setOverride] = useState<boolean | null>(null);
  const [systemReduced, setSystemReduced] = useState(true);
  useEffect(() => {
    const preference = matchMedia("(prefers-reduced-motion: reduce)");
    const update = () => setSystemReduced(preference.matches);
    update(); preference.addEventListener("change", update);
    return () => preference.removeEventListener("change", update);
  }, []);
  const reduced = override ?? systemReduced;
  useEffect(() => {
    const root = ref.current;
    const foundArt = root?.querySelector<HTMLElement>(".friends-art");
    const rig = foundArt?.querySelector<HTMLElement>(".friends-art-rig");
    if (!root || !foundArt || !rig) return;
    const art = foundArt;
    const circles = [...art.querySelectorAll<HTMLElement>(".friends-circle")];
    const mouse = matchMedia("(hover: hover) and (pointer: fine)");
    let tweens: Animation[] = [], base: string[] = [];
    let version = 0, hovering = false;
    function reset() { ++version; tweens.forEach(a => { a.cancel(); }); tweens = []; base = []; hovering = false; delete art.dataset.attracted; }
    function attract(together: boolean) {
      if (reduced || !mouse.matches || document.hidden || art.dataset.visible === "false" || together === hovering) return;
      hovering = together;
      const current = circles.map(c => getComputedStyle(c).transform);
      if (!base.length) base = current;
      const run = ++version;
      tweens.forEach(a => { a.cancel(); }); art.dataset.attracted = "true";
      tweens = circles.map((c, i) => c.animate([{ transform: current[i] }, { transform: together ? `translateX(${getComputedStyle(c).getPropertyValue("--friends-meet-shift").trim()})` : base[i] }], { duration: together ? 650 : 750, easing: "cubic-bezier(.22,.61,.36,1)", fill: "forwards" }));
      Promise.all(tweens.map(a => a.finished)).then(() => { if (run === version && !together) reset(); }).catch(() => { /* Superseded pointer transition. */ });
    }
    const enter = (event: PointerEvent) => { if (event.pointerType === "mouse") attract(true); };
    const leave = (event: PointerEvent) => { if (event.pointerType === "mouse") attract(false); };
    const visibility = () => { root.dataset.documentHidden = String(document.hidden); if (document.hidden) reset(); };
    visibility(); circles.forEach(c => { c.addEventListener("pointerenter", enter); });
    rig.addEventListener("pointerleave", leave); rig.addEventListener("pointercancel", reset);
    mouse.addEventListener("change", reset); window.addEventListener("blur", reset); document.addEventListener("visibilitychange", visibility);
    const observer = new IntersectionObserver(entries => { art.dataset.visible = String(entries[0].isIntersecting); if (!entries[0].isIntersecting) reset(); }); observer.observe(art);
    return () => { reset(); observer.disconnect(); circles.forEach(c => { c.removeEventListener("pointerenter", enter); }); rig.removeEventListener("pointerleave", leave); rig.removeEventListener("pointercancel", reset); mouse.removeEventListener("change", reset); window.removeEventListener("blur", reset); document.removeEventListener("visibilitychange", visibility); };
  }, [reduced]);
  return <MotionPreference.Provider value={{ reduced, setReduced: setOverride }}><div className="friends-page" ref={ref} data-motion={reduced ? "off" : "on"}>{children}</div></MotionPreference.Provider>;
}
