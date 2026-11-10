/* Local review prototype. Production integration will use React-scoped cleanup. */
(() => {
  const frame = document.getElementById("viewport");
  const hero = frame.querySelector(".hero");
  const replay = document.getElementById("replay-motion");
  const reduce = document.getElementById("reduce-motion");
  const status = document.getElementById("preview-status");
  const motionStatus = document.getElementById("motion-status");
  const preference = window.matchMedia("(prefers-reduced-motion: reduce)");
  const { gsap, ScrollTrigger } = window;
  let context;
  let entrance;
  let pendingFrame = 0;
  let previousWidth = frame.clientWidth;
  let disposed = false;
  let manualReduced = reduce.checked;

  function describeMotion(message) {
    motionStatus.textContent = message;
    status.textContent = message;
  }

  if (!gsap || !ScrollTrigger) {
    replay.disabled = true;
    reduce.disabled = true;
    describeMotion("Static preview: the local GSAP or ScrollTrigger file could not load. Open the complete prototype folder, including vendor/. Content and navigation remain available.");
    return;
  }
  gsap.registerPlugin(ScrollTrigger);

  function rebuild(animateEntrance = false) {
    if (disposed) return;
    context?.revert();
    entrance = null;
    const reduced = preference.matches || manualReduced;
    reduce.checked = reduced;
    reduce.disabled = preference.matches;
    replay.disabled = reduced;
    describeMotion(preference.matches
      ? "Your system requests reduced motion. The preview is static and Replay is disabled. Enable animations in your system settings to preview movement."
      : manualReduced
        ? "Reduced motion is on for this preview. Uncheck Reduce motion to enable Replay and animations."
        : "Vivid motion enabled. Use Replay entrance to restart the hero, then scroll to see the graphics move.");
    if (reduced) return;

    const mobile = frame.clientWidth <= 620;
    const scroller = document.fullscreenElement === frame ? frame : window;
    try {
      context = gsap.context(() => {}, frame);
      context.add(() => {
        if (animateEntrance && !document.hidden) {
          entrance = gsap.timeline({ defaults: { ease: "power3.out" } });
          entrance.from(".graphic span", {
            scale: 0.45, opacity: 0, rotation: -18,
            duration: mobile ? 0.6 : 0.8, stagger: 0.04,
            transformOrigin: "50% 50%",
          }, 0);
          entrance.from(".hero h2 span", {
            yPercent: 32, opacity: 0, duration: 0.65, stagger: 0.07,
          }, 0.12);
        }

        gsap.to(".graphic", {
          y: () => -(mobile ? 35 : 90), rotation: mobile ? 3 : 8,
          ease: "none",
          scrollTrigger: {
            trigger: hero, scroller, start: "top top", end: "bottom top",
            scrub: 0.5, invalidateOnRefresh: true,
          },
        });

        frame.querySelectorAll(".article:not([hidden])").forEach(article => {
          const reveal = gsap.from(article, {
            y: mobile ? 10 : 18, opacity: 0, duration: 0.45,
            ease: "power2.out", immediateRender: false,
            scrollTrigger: { trigger: article, scroller, start: "top 92%", once: true },
          });
          // Keyboard focus makes an article fully visible immediately.
          const expose = () => {
            reveal.progress(1).pause();
            reveal.scrollTrigger?.kill();
          };
          article.addEventListener("focusin", expose);
          context.add(() => () => article.removeEventListener("focusin", expose));
        });
      });
      ScrollTrigger.refresh();
    } catch {
      context?.revert();
      replay.disabled = true;
      describeMotion("Static preview: animation setup failed. Content and navigation remain available. Reload the complete prototype folder to try again.");
    }
  }

  function scheduleRebuild() {
    cancelAnimationFrame(pendingFrame);
    pendingFrame = requestAnimationFrame(() => rebuild(false));
  }
  function onReplay() {
    if (preference.matches || manualReduced) return;
    cancelAnimationFrame(pendingFrame);
    previousWidth = frame.clientWidth;
    hero.scrollIntoView({ behavior: "auto", block: "start" });
    rebuild(true);
  }
  function onFullscreen() {
    cancelAnimationFrame(pendingFrame);
    previousWidth = frame.clientWidth;
    rebuild(document.fullscreenElement === frame);
  }
  function onVisibility() {
    if (document.hidden) entrance?.pause();
    else entrance?.resume();
  }
  const observer = new ResizeObserver(() => {
    const width = frame.clientWidth;
    if (width !== previousWidth) {
      previousWidth = width;
      scheduleRebuild();
    }
  });
  observer.observe(frame);
  replay.addEventListener("click", onReplay);
  reduce.addEventListener("change", () => {
    manualReduced = reduce.checked;
    scheduleRebuild();
  });
  preference.addEventListener("change", scheduleRebuild);
  document.addEventListener("fullscreenchange", onFullscreen);
  document.addEventListener("visibilitychange", onVisibility);
  document.getElementById("article-state").addEventListener("change", scheduleRebuild);

  // Review notes live outside the full-screen frame. Exit that mode before
  // following a prototype navigation link to its explanatory destination.
  frame.querySelectorAll('a[href^="#"]').forEach(link => {
    link.addEventListener("click", async event => {
      const target = document.getElementById(link.hash.slice(1));
      if (target && !frame.contains(target) && document.fullscreenElement === frame) {
        event.preventDefault();
        try {
          await document.exitFullscreen();
          target.setAttribute("tabindex", "-1");
          target.focus({ preventScroll: true });
          target.scrollIntoView({ behavior: "auto", block: "start" });
        } catch { status.textContent = "Press Escape to leave full screen and read the design notes."; }
      }
    });
  });
  window.addEventListener("pagehide", () => {
    disposed = true;
    cancelAnimationFrame(pendingFrame);
    context?.revert();
    observer.disconnect();
  });
  window.addEventListener("pageshow", event => {
    if (event.persisted) {
      disposed = false;
      observer.observe(frame);
      rebuild(false);
    }
  });
  rebuild(true);
})();
