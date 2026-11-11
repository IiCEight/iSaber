"use client";

import { useEffect, useRef, useState } from "react";
import type { ArticleHeading } from "../lib/article-outline";

export default function ArticleContents({ headings }: { headings: ArticleHeading[] }) {
  const details = useRef<HTMLDetailsElement>(null);
  const [active, setActive] = useState("");
  const visibleHeadings = headings.filter(heading => heading.depth <= 3);
  useEffect(() => {
    const small = matchMedia("(max-width: 1000px)");
    const resize = () => { if (details.current) details.current.open = !small.matches; };
    resize(); small.addEventListener("change", resize);
    const ids = [...headings.filter(heading => heading.depth <= 3).map(heading => heading.id), "comments-title"];
    const visible = new Set<string>();
    const observer = new IntersectionObserver(entries => {
      for (const entry of entries) { if (entry.isIntersecting) visible.add(entry.target.id); else visible.delete(entry.target.id); }
      setActive(ids.find(id => visible.has(id)) ?? "");
    }, { rootMargin: "-5% 0px -55% 0px" });
    for (const id of ids) { const node = document.getElementById(id); if (node) observer.observe(node); }
    return () => { observer.disconnect(); small.removeEventListener("change", resize); };
  }, [headings]);
  return <aside className="article-contents">
    <details ref={details} open><summary>On this page</summary><nav aria-label="Table of contents">
      {visibleHeadings.map((heading, index) => <a key={heading.id} href={`#${heading.id}`} className={heading.depth === 3 ? "article-contents-subsection" : undefined} aria-current={active === heading.id ? "location" : undefined}><span aria-hidden="true">{String(index + 1).padStart(2, "0")}</span>{heading.text}</a>)}
      <a href="#comments-title" aria-current={active === "comments-title" ? "location" : undefined}><span aria-hidden="true">↘</span>Conversation</a>
    </nav></details><a className="article-contents-back" href="#content">Back to top ↑</a>
  </aside>;
}
