"use client";

import { useEffect, useRef, type ReactNode } from "react";
import { gsap } from "gsap";
import { ScrollTrigger } from "gsap/ScrollTrigger";

export default function HomeMotion({ children }: { children: ReactNode }) {
  const root = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const element = root.current;
    if (!element) return;
    gsap.registerPlugin(ScrollTrigger);
    const preference = matchMedia("(prefers-reduced-motion: reduce)");
    const mobile = matchMedia("(max-width: 620px)");
    let context: gsap.Context | undefined;
    let entrance: gsap.core.Timeline | undefined;
    function setup(animateEntrance = false) {
      context?.revert();
      entrance = undefined;
      if (preference.matches || !element) return;
      try {
        context = gsap.context(() => {}, element);
        context.add(() => {
          if (animateEntrance && !document.hidden) {
            entrance = gsap.timeline({ defaults: { ease: "power3.out" } });
            entrance.from(".home-graphic span", { scale: 0.45, opacity: 0, rotation: -18, duration: mobile.matches ? 0.6 : 0.8, stagger: 0.04 }, 0);
            entrance.from(".home-title span", { yPercent: 32, opacity: 0, duration: 0.65, stagger: 0.07 }, 0.12);
          }
          gsap.to(".home-graphic", {
            y: () => mobile.matches ? -35 : -90, rotation: mobile.matches ? 3 : 8, ease: "none",
            scrollTrigger: { trigger: element.querySelector(".home-hero"), start: "top top", end: "bottom top", scrub: 0.5, invalidateOnRefresh: true },
          });
          element.querySelectorAll<HTMLElement>("[data-home-article]").forEach(article => {
            const reveal = gsap.from(article, {
              y: mobile.matches ? 10 : 18, opacity: 0, duration: 0.45, ease: "power2.out", immediateRender: false,
              scrollTrigger: { trigger: article, start: "top 92%", once: true },
            });
            const expose = () => { reveal.progress(1).pause(); reveal.scrollTrigger?.kill(); };
            article.addEventListener("focusin", expose);
            context?.add(() => () => article.removeEventListener("focusin", expose));
          });
        });
      } catch {
        // Animation failure must leave server-rendered content visible.
        context?.revert();
      }
    }
    const rebuild = () => setup(false);
    const visibility = () => { if (document.hidden) entrance?.pause(); else entrance?.resume(); };
    preference.addEventListener("change", rebuild);
    mobile.addEventListener("change", rebuild);
    document.addEventListener("visibilitychange", visibility);
    setup(true);
    return () => {
      context?.revert();
      preference.removeEventListener("change", rebuild);
      mobile.removeEventListener("change", rebuild);
      document.removeEventListener("visibilitychange", visibility);
    };
  }, []);
  return <div className="home-page" ref={root}>{children}</div>;
}
