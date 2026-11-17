"use client";
import { useEffect, useRef } from "react";

export default function CategoryMotion() {
 const ref = useRef<HTMLDivElement>(null);
 useEffect(() => {
  const element = ref.current;
  if (!element) return;
  const visibility = () => { element.dataset.hidden = String(document.hidden); };
  visibility(); document.addEventListener("visibilitychange", visibility);
  const observer = typeof IntersectionObserver === "undefined" ? null : new IntersectionObserver(entries => { element.dataset.visible = String(entries[0].isIntersecting); });
  observer?.observe(element);
  return () => { observer?.disconnect(); document.removeEventListener("visibilitychange", visibility); };
 }, []);
 return <div className="category-art" aria-hidden="true" ref={ref}><div className="category-art-rig">{["one", "two", "three", "four"].map(name => <span key={name} className={`category-circle category-circle-${name}`} />)}</div></div>;
}
