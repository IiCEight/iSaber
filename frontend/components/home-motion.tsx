"use client";

import { useEffect, useRef, type ReactNode } from "react";
import { gsap } from "gsap";
import { ScrollTrigger } from "gsap/ScrollTrigger";

function attractCircles(root: HTMLElement) {
  const groups = Array.from(root.querySelectorAll<HTMLElement>(".home-orbit"), orbit => ({
    orbit, circles: Array.from(orbit.querySelectorAll<HTMLElement>("span"), node => ({ node, x: 0, y: 0 })),
  }));
  const finePointer = matchMedia("(hover: hover) and (pointer: fine)");
  let pointer: { x: number; y: number } | undefined;
  let ticking = false;
  const tick = (_time: number, delta: number) => {
    const smoothing = 1 - Math.exp(-Math.min(delta, 64) / 150);
    // Read geometry before writing transforms. Remove the previous offset so
    // attraction is measured from each circle's resting center, without drift.
    const updates = groups.flatMap(({ orbit, circles }) => {
      const matrix = new DOMMatrix(getComputedStyle(orbit).transform);
      const inverse = matrix.inverse();
      const visible = getComputedStyle(orbit).visibility !== "hidden";
      return circles.map(circle => {
        const rect = circle.node.getBoundingClientRect();
        const dx = pointer ? pointer.x - (rect.x + rect.width / 2 - matrix.a * circle.x - matrix.c * circle.y) : 0;
        const dy = pointer ? pointer.y - (rect.y + rect.height / 2 - matrix.b * circle.x - matrix.d * circle.y) : 0;
        const distance = Math.hypot(dx, dy);
        // Include the visible edge of large lenses, rather than limiting their
        // response to a small central patch. Smoothly taper the outer range.
        const radius = Math.max(rect.width, rect.height) / 2 + 180;
        const proximity = Math.min(1, distance / radius);
        const strength = pointer && visible ? 1 - proximity * proximity * (3 - 2 * proximity) : 0;
        const amount = Math.min(28, distance * 0.35) * strength;
        const screenX = distance ? dx / distance * amount : 0;
        const screenY = distance ? dy / distance * amount : 0;
        return { circle, x: inverse.a * screenX + inverse.c * screenY, y: inverse.b * screenX + inverse.d * screenY };
      });
    });
    let resting = true;
    for (const { circle, x, y } of updates) {
      circle.x += (x - circle.x) * smoothing;
      circle.y += (y - circle.y) * smoothing;
      circle.node.style.setProperty("--lens-x", `${circle.x}px`);
      circle.node.style.setProperty("--lens-y", `${circle.y}px`);
      if (Math.abs(circle.x) + Math.abs(circle.y) > 0.02) resting = false;
    }
    if (!pointer && resting) { gsap.ticker.remove(tick); ticking = false; }
  };
  const move = (event: PointerEvent) => {
    if (!finePointer.matches || event.pointerType !== "mouse") return;
    pointer = { x: event.clientX, y: event.clientY };
    if (!ticking) { ticking = true; gsap.ticker.add(tick); }
  };
  const leave = () => { pointer = undefined; };
  const visibility = () => { if (document.hidden) leave(); };
  root.addEventListener("pointermove", move, { passive: true });
  root.addEventListener("pointerleave", leave);
  window.addEventListener("blur", leave);
  finePointer.addEventListener("change", leave);
  document.addEventListener("visibilitychange", visibility);
  return () => {
    gsap.ticker.remove(tick);
    root.removeEventListener("pointermove", move);
    root.removeEventListener("pointerleave", leave);
    window.removeEventListener("blur", leave);
    finePointer.removeEventListener("change", leave);
    document.removeEventListener("visibilitychange", visibility);
    for (const { circles } of groups) for (const { node } of circles) {
      node.style.removeProperty("--lens-x"); node.style.removeProperty("--lens-y");
    }
  };
}

export default function HomeMotion({ children }: { children: ReactNode }) {
  const root = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const element = root.current;
    const story = element?.querySelector<HTMLElement>(".home-story");
    const writing = element?.querySelector<HTMLElement>(".home-content");
    if (!element || !story || !writing) return;
    gsap.registerPlugin(ScrollTrigger);
    const preference = matchMedia("(prefers-reduced-motion: reduce)");
    const mobile = matchMedia("(max-width: 620px)");
    const shortViewport = matchMedia("(max-height: 620px)");
    let context: gsap.Context | undefined;
    let entrance: gsap.core.Timeline | undefined;
    let scene: gsap.core.Timeline | undefined;
    let stopAttraction: (() => void) | undefined;
    const clearAttraction = () => { stopAttraction?.(); stopAttraction = undefined; };

    function setup(animateEntrance = false) {
      context?.revert();
      // Clear pointer offsets after GSAP restores its saved inline styles.
      clearAttraction();
      story?.classList.remove("is-animated");
      entrance = undefined;
      scene = undefined;
      // The default document is a complete, linear version of the story.
      // Short/zoomed viewports also use it so cards never get trapped in a crop.
      if (preference.matches || shortViewport.matches || parseFloat(getComputedStyle(document.documentElement).fontSize) > 20 || !element || !story || !writing) return;
      try {
        context = gsap.context(() => {}, element);
        context.add(() => {
          story.classList.add("is-animated");
          context?.add(() => () => story.classList.remove("is-animated"));
          stopAttraction = attractCircles(element);
          if (animateEntrance && !document.hidden && window.scrollY < 100) {
            entrance = gsap.timeline({ defaults: { ease: "power3.out" } });
            // A GSAP scale tween normalizes CSS translate/rotate/scale into an
            // inline transform, disabling pointer translation and breathing.
            // Animate a separate variable so those CSS properties stay live.
            entrance.fromTo(".home-orbit--one span", { "--lens-entrance": 0.45, opacity: 0 }, { "--lens-entrance": 1, opacity: 0.85, duration: mobile.matches ? 0.6 : 0.8, stagger: 0.03 }, 0);
            entrance.from(".home-title span", { yPercent: 32, opacity: 0, duration: 0.65, stagger: 0.07 }, 0.12);
          }

          scene = gsap.timeline({
            defaults: { ease: "power1.inOut" },
            scrollTrigger: {
              trigger: story, start: "top top",
              // End the composition at the reading entrance, independent of
              // how many articles are returned. The list then scrolls normally.
              end: () => `+=${Math.max(innerHeight, writing.offsetTop - (mobile.matches ? 84 : 112))}`,
              scrub: 0.6, invalidateOnRefresh: true,
            },
          });
          scene.to(".home-hero", { autoAlpha: 0, yPercent: -18, duration: 0.55 }, 0.25)
            .to(".home-warm-pane", { width: "100%", height: "74%", duration: 1 }, 0.2)
            // Slide the complete backdrop beyond the clipped canvas edge.
            .to(".home-ink-pane", { left: "100%", duration: 1.05 }, 0.2)
            .to(".home-orbit--one", { x: () => -element.clientWidth * (mobile.matches ? 0.28 : 0.3), y: () => -innerHeight * 0.06, rotation: -48, scale: 0.9, duration: 1.1 }, 0.15)
            .fromTo(".home-story-intro", { autoAlpha: 0, yPercent: 12 }, { autoAlpha: 1, yPercent: 0, duration: 0.6 }, 0.65)
            .to(".home-story-intro", { autoAlpha: 0, yPercent: -12, duration: 0.5 }, 1.65)
            .to(".home-warm-pane", { autoAlpha: 0, duration: 0.6 }, 1.65)
            .to(".home-orbit--one", { left: mobile.matches ? "-34%" : "-3%", top: mobile.matches ? "8%" : "-22%", x: 0, y: 0, rotation: 135, scale: mobile.matches ? 0.62 : 0.67, duration: 1.2 }, 1.45)
            .fromTo(".home-orbit--two", { autoAlpha: 0, scale: 0.45, rotation: 25 }, { autoAlpha: 1, scale: 1, rotation: -90, duration: 1.1 }, 1.55)
            .fromTo(".home-story-invitation", { autoAlpha: 0, yPercent: 12 }, { autoAlpha: 1, yPercent: 0, duration: 0.65 }, 1.95)
            .to(".home-story-invitation", { autoAlpha: 1, duration: 0.3 }, 2.6)
            .to(".home-story-invitation", { autoAlpha: 0, yPercent: -12, duration: 0.5 }, 2.85)
            .to(".home-orbit--one", { left: mobile.matches ? "-88%" : "-40%", top: mobile.matches ? "4%" : "2%", rotation: 220, scale: mobile.matches ? 0.6 : 0.63, duration: 1.1 }, 2.9)
            .to(".home-orbit--two", { left: mobile.matches ? "58%" : "70%", top: mobile.matches ? "54%" : "-6%", rotation: -170, duration: 1.1 }, 2.9)
            .to(".home-orbit--three", { top: "65%", left: "32%", scale: 0.72, rotation: 120, duration: 1.1 }, 2.9)
            .fromTo(".home-section-heading", { y: 32 }, { y: 0, duration: 0.9 }, 3.1)
            .to(".home-section-heading", { y: 0, duration: 0.2 }, 4);
          if (!mobile.matches) scene.fromTo(".home-orbit--three", { autoAlpha: 0, scale: 0.5, rotation: -20 }, { autoAlpha: 1, scale: 0.9, rotation: 70, duration: 1 }, 1.7);

          gsap.to(".home-graphic", {
            y: mobile.matches ? -35 : -70, ease: "none",
            scrollTrigger: { trigger: writing, start: "top bottom", end: "bottom bottom", scrub: 0.6, invalidateOnRefresh: true },
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
        ScrollTrigger.refresh();
      } catch {
        context?.revert();
        clearAttraction();
        story.classList.remove("is-animated");
      }
    }
    const rebuild = () => {
      const before = writing.getBoundingClientRect();
      const readingInView = before.top < innerHeight && before.bottom > 72;
      setup(false);
      if (readingInView) window.scrollBy({ top: writing.getBoundingClientRect().top - before.top, behavior: "auto" });
    };
    let fontSize = getComputedStyle(document.documentElement).fontSize;
    const textObserver = new ResizeObserver(() => {
      const nextSize = getComputedStyle(document.documentElement).fontSize;
      if (nextSize !== fontSize) {
        fontSize = nextSize;
        rebuild();
      }
    });
    const introCard = element.querySelector(".home-intro-card");
    if (introCard) textObserver.observe(introCard);
    const visibility = () => { if (document.hidden) entrance?.pause(); else entrance?.resume(); };
    const jumpToWriting = (event: MouseEvent) => {
      const target = event.target;
      if (!(target instanceof Element) || !target.closest('a[href="#recent-articles"]')) return;
      // Finish the scrub before a native anchor jump, rather than animating
      // through intermediate panels underneath the reader's new position.
      scene?.progress(1);
    };
    const restoreAnchor = () => {
      if (location.hash === "#recent-articles") {
        scene?.progress(1);
        document.getElementById("recent-articles")?.scrollIntoView({ behavior: "auto", block: "start" });
        return;
      }
      if (location.hash !== "#home-title") return;
      // A heading inside a sticky scene is not a stable document-top anchor.
      history.replaceState(history.state, "", `${location.pathname}${location.search}#content`);
      window.scrollTo({ top: 0, behavior: "auto" });
    };
    preference.addEventListener("change", rebuild);
    mobile.addEventListener("change", rebuild);
    shortViewport.addEventListener("change", rebuild);
    document.addEventListener("visibilitychange", visibility);
    element.addEventListener("click", jumpToWriting);
    window.addEventListener("hashchange", restoreAnchor);
    window.addEventListener("load", restoreAnchor);
    setup(true);
    restoreAnchor();
    return () => {
      context?.revert();
      clearAttraction();
      story.classList.remove("is-animated");
      textObserver.disconnect();
      preference.removeEventListener("change", rebuild);
      mobile.removeEventListener("change", rebuild);
      shortViewport.removeEventListener("change", rebuild);
      document.removeEventListener("visibilitychange", visibility);
      element.removeEventListener("click", jumpToWriting);
      window.removeEventListener("hashchange", restoreAnchor);
      window.removeEventListener("load", restoreAnchor);
    };
  }, []);
  return <div className="home-page" ref={root}>{children}</div>;
}
