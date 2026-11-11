"use client";

import { createContext, useContext, useEffect, useRef, useState, type ReactNode } from "react";
import { gsap } from "gsap";
import { ScrollTrigger } from "gsap/ScrollTrigger";

const MotionPreference = createContext<{ reduced: boolean; setReduced: (value: boolean) => void } | null>(null);
export function AboutMotionControl() {
  const preference = useContext(MotionPreference);
  const [ready, setReady] = useState(false);
  useEffect(() => setReady(true), []);
  if (!preference || !ready) return null;
  return <label className="about-motion-control"><input type="checkbox" checked={preference.reduced} onChange={event => preference.setReduced(event.target.checked)} />Reduce motion</label>;
}

export default function AboutMotion({ children }: { children: ReactNode }) {
  const root = useRef<HTMLDivElement>(null);
  const [reduced, setReduced] = useState(false);
  useEffect(() => {
    const element = root.current;
    const story = element?.querySelector<HTMLElement>(".about-story");
    if (!element || !story) return;
    gsap.registerPlugin(ScrollTrigger);
    const preference = matchMedia("(prefers-reduced-motion: reduce)");
    const mobile = matchMedia("(max-width: 620px)");
    const short = matchMedia("(max-height: 620px)");
    const finePointer = matchMedia("(hover: hover) and (pointer: fine)");
    let context: gsap.Context | undefined;
    let entrance: gsap.core.Timeline | undefined;
    let cleanup: (() => void)[] = [];
    let frame = 0;
    const headerHeight = () => document.querySelector(".site-header")?.getBoundingClientRect().height ?? 72;
    const clear = () => {
      for (const stop of cleanup) stop();
      cleanup = [];
      context?.revert();
      context = undefined;
      story.classList.remove("is-motion");
      entrance = undefined;
    };
    const setup = (enter = false) => {
      clear();
      if (preference.matches || reduced || short.matches || parseFloat(getComputedStyle(document.documentElement).fontSize) > 20) return;
      try {
        context = gsap.context(() => {}, story);
        context.add(() => {
          story.classList.add("is-motion");
          const slots = gsap.utils.toArray<HTMLElement>(".petal-slot");
          const scatter = [[20,10],[56,8],[88,20],[14,44],[48,38],[84,48],[4,78],[39,76],[75,83],[100,72],[55,104],[12,109]];
          const scene = gsap.timeline({ defaults: { ease: "power1.inOut" }, scrollTrigger: { trigger: story, start: () => `top top+=${headerHeight()}`, end: "bottom bottom", scrub: .65, invalidateOnRefresh: true } });
          scene.to(".warm-field", { width: "100%", duration: .95, ease: "sine.inOut" }, .05)
            .to(".orbit-layout", { left: mobile.matches ? "24%" : "37%", top: mobile.matches ? "6%" : "-10%", width: mobile.matches ? "115vmin" : "105vmin", duration: 1 }, 0)
            .to(".orbit-rig", { rotation: 72, duration: 1 }, 0)
            .to(slots, { left: (i: number) => `${scatter[i][0]}%`, top: (i: number) => `${scatter[i][1]}%`, width: mobile.matches ? "27%" : "28%", duration: 1 }, .1)
            .to(".warm-field", { xPercent: -48, duration: .7, ease: "sine.inOut" }, 1.6)
            .to(".orbit-layout", { left: mobile.matches ? "-20%" : "-9%", top: mobile.matches ? "22%" : "4%", width: mobile.matches ? "125vmin" : "95vmin", duration: 1 }, 1.05)
            .to(".orbit-rig", { rotation: 148, duration: 1 }, 1.05)
            .to(slots, { left: (i: number) => `${16 + (i % 4) * 24}%`, top: (i: number) => `${20 + Math.floor(i / 4) * 28}%`, width: "34%", duration: 1 }, 1.1)
            .to(".warm-field", { xPercent: -102, duration: .75, ease: "sine.inOut" }, 2.3)
            .set(".warm-field", { autoAlpha: 0 }, 3.05)
            .to(".orbit-layout", { left: mobile.matches ? "45%" : "64%", top: mobile.matches ? "7%" : "-12%", width: mobile.matches ? "115vmin" : "88vmin", duration: 1 }, 2.1)
            .to(".orbit-rig", { rotation: 240, duration: 1 }, 2.1)
            .to(slots, { left: (i: number) => `${50 + Math.cos(i * Math.PI / 6) * 30}%`, top: (i: number) => `${50 + Math.sin(i * Math.PI / 6) * 30}%`, width: "34%", duration: 1 }, 2.1)
            .to(".orbit-rig", { rotation: 260, duration: .4 }, 3.1);
          if (enter && !document.hidden && scrollY < 100) {
            entrance = gsap.timeline();
            entrance.from(".title-line > span", { yPercent: 24, opacity: 0, duration: .85, stagger: .12, ease: "power3.out" }, .08)
              .from(".chapter-label", { opacity: 0, duration: .6 }, .1);
          }
          story.querySelectorAll<HTMLElement>(".interest-row").forEach(row => {
            const reveal = gsap.from(row, { y: 24, opacity: 0, duration: .65, ease: "power2.out", immediateRender: false, scrollTrigger: { trigger: row, start: "top 92%", once: true } });
            const expose = () => { reveal.progress(1).pause(); reveal.scrollTrigger?.kill(); };
            row.addEventListener("focusin", expose); cleanup.push(() => row.removeEventListener("focusin", expose));
          });
          const pointer = story.querySelector(".pointer-layer");
          const x = gsap.quickTo(pointer, "x", { duration: .7, ease: "power3.out" });
          const y = gsap.quickTo(pointer, "y", { duration: .7, ease: "power3.out" });
          const leave = () => { x(0); y(0); };
          const move = (event: PointerEvent) => {
            if (!finePointer.matches || event.pointerType !== "mouse") return;
            x(gsap.utils.clamp(-14, 14, (event.clientX / innerWidth - .5) * 28));
            y(gsap.utils.clamp(-10, 10, (event.clientY / innerHeight - .5) * 20));
          };
          story.addEventListener("pointermove", move, { passive: true }); story.addEventListener("pointerleave", leave); window.addEventListener("blur", leave);
          cleanup.push(() => { story.removeEventListener("pointermove", move); story.removeEventListener("pointerleave", leave); window.removeEventListener("blur", leave); });
        });
        ScrollTrigger.refresh();
      } catch { clear(); }
    };
    const rebuild = () => setup(false);
    const visibility = () => {
      element.classList.toggle("about-motion-paused", document.hidden);
      if (document.hidden) entrance?.pause(); else entrance?.resume();
    };
    let fontSize = getComputedStyle(document.documentElement).fontSize;
    const observer = new ResizeObserver(() => {
      const next = getComputedStyle(document.documentElement).fontSize;
      if (next !== fontSize) { fontSize = next; rebuild(); }
    });
    const hero = story.querySelector(".hero-copy");
    if (hero) observer.observe(hero);
    const restoreAnchor = () => {
      let target: HTMLElement | null;
      try { target = location.hash ? document.getElementById(decodeURIComponent(location.hash.slice(1))) : null; } catch { return; }
      if (target && (element.contains(target) || target.id === "content")) { target.scrollIntoView({ block: "start" }); ScrollTrigger.update(); }
    };
    preference.addEventListener("change", rebuild); mobile.addEventListener("change", rebuild); short.addEventListener("change", rebuild);
    document.addEventListener("visibilitychange", visibility); window.addEventListener("load", restoreAnchor); window.addEventListener("hashchange", restoreAnchor);
    setup(true); visibility(); frame = requestAnimationFrame(restoreAnchor);
    return () => {
      cancelAnimationFrame(frame); clear(); observer.disconnect(); element.classList.remove("about-motion-paused");
      preference.removeEventListener("change", rebuild); mobile.removeEventListener("change", rebuild); short.removeEventListener("change", rebuild);
      document.removeEventListener("visibilitychange", visibility); window.removeEventListener("load", restoreAnchor); window.removeEventListener("hashchange", restoreAnchor);
    };
  }, [reduced]);
  return <MotionPreference.Provider value={{ reduced, setReduced }}><div className="about-page" ref={root}>{children}</div></MotionPreference.Provider>;
}
