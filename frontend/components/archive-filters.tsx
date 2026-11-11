"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState, useTransition } from "react";
import type { Label } from "../lib/articles";

const fields = ["q", "category", "tag", "language"] as const;
export default function ArchiveFilters({ query, categories, tags, labelsUnavailable }: {
  query: string; categories: Label[]; tags: Label[]; labelsUnavailable: boolean;
}) {
  const router = useRouter();
  const [enhanced, setEnhanced] = useState(false);
  const [open, setOpen] = useState(false);
  const [pending, startTransition] = useTransition();
  const opener = useRef<HTMLButtonElement>(null);
  const form = useRef<HTMLFormElement>(null);
  const input = useRef<HTMLInputElement>(null);
  const applied = new URLSearchParams(query);
  // Server HTML exposes the complete GET form even without JavaScript.
  useEffect(() => setEnhanced(true), []);
  const close = () => { setOpen(false); opener.current?.focus({ preventScroll: true }); };
  const expand = () => {
    form.current?.reset();
    setOpen(true);
    requestAnimationFrame(() => input.current?.focus({ preventScroll: true }));
  };
  const without = (key: string) => {
    const next = new URLSearchParams(query); next.delete(key); next.delete("cursor");
    return `/archive${next.size ? `?${next}` : ""}#archive-writing`;
  };
  const active = fields.filter(key => applied.get(key));
  const names = { q: "Search", category: "Category", tag: "Tag", language: "Language" };
  const valueLabel = (key: typeof fields[number]) => {
    const value = applied.get(key) ?? "";
    if (key === "category") return categories.find(label => label.slug === value)?.name ?? value;
    if (key === "tag") return tags.find(label => label.slug === value)?.name ?? value;
    if (key === "language") return value === "en" ? "English" : value === "zh" ? "中文" : value;
    return value;
  };
  return <>
    <div className="archive-index-top" id="archive-writing">
      <h2>All writing</h2>
      <button ref={opener} className="archive-search-toggle" type="button" hidden={!enhanced} aria-label="Search and filters" aria-expanded={open} aria-controls="archive-search-panel" onClick={() => open ? close() : expand()}>
        <svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" strokeWidth="1.6" aria-hidden="true"><circle cx="10.5" cy="10.5" r="6.5" /><path d="m15.5 15.5 5 5" /></svg>
        {active.length > 0 && <span className="archive-filter-dot" aria-hidden="true" />}
      </button>
    </div>
    <section id="archive-search-panel" className={`archive-search-panel${enhanced && !open ? " is-collapsed" : ""}`} aria-label="Search and filter articles" aria-hidden={enhanced && !open} inert={enhanced && !open} onKeyDown={event => { if (event.key === "Escape" && enhanced) { event.preventDefault(); close(); } }}>
      <div className="archive-search-inner"><div className="archive-search-surface">
        <div className="archive-panel-heading"><h3>Search &amp; filter</h3><button type="button" aria-label="Collapse search" hidden={!enhanced} onClick={close}>×</button></div>
        <form ref={form} key={query} action="/archive#archive-writing" method="get" className="archive-tools" aria-busy={pending} onSubmit={event => {
          event.preventDefault();
          const data = new FormData(event.currentTarget), next = new URLSearchParams();
          for (const key of fields) { const value = String(data.get(key) ?? "").trim(); if (value) next.set(key, value); }
          close();
          startTransition(() => router.push(`/archive${next.size ? `?${next}` : ""}#archive-writing`));
        }}>
          <search aria-label="Search public articles"><div className="archive-search-line">
            <label htmlFor="archive-query">Search articles<input ref={input} id="archive-query" type="search" name="q" defaultValue={applied.get("q") ?? ""} maxLength={200} placeholder="Search the collection…" /></label>
            <button className="archive-solid" type="submit" disabled={pending}>Search ↗</button>
          </div><p className="archive-search-help">Search titles, summaries, tags, and article text. All terms must match.</p></search>
          <div className="archive-filter-row">
            <label>Category<select name="category" defaultValue={applied.get("category") ?? ""}>
              <option value="">All categories</option>{applied.has("category") && !categories.some(label => label.slug === applied.get("category")) && <option value={applied.get("category") ?? ""}>Selected category: {applied.get("category")}</option>}
              {categories.map(label => <option key={label.id} value={label.slug}>{label.name}</option>)}
            </select></label>
            <label>Tag<select name="tag" defaultValue={applied.get("tag") ?? ""}>
              <option value="">All tags</option>{applied.has("tag") && !tags.some(label => label.slug === applied.get("tag")) && <option value={applied.get("tag") ?? ""}>Selected tag: {applied.get("tag")}</option>}
              {tags.map(label => <option key={label.id} value={label.slug}>{label.name}</option>)}
            </select></label>
            <label>Language<select name="language" defaultValue={applied.get("language") ?? ""}>
              <option value="">English &amp; 中文</option><option value="en">English</option><option value="zh">中文</option>
              {applied.has("language") && !["en", "zh"].includes(applied.get("language") ?? "") && <option value={applied.get("language") ?? ""}>Selected language: {applied.get("language")}</option>}
            </select></label>
            <button className="archive-clear" type="reset">Reset fields</button>
          </div>
          {labelsUnavailable && <p className="archive-search-help">Some filter options are temporarily unavailable. Your selected conditions are preserved.</p>}
          <div className="archive-panel-actions"><Link href="/archive#archive-writing" onClick={close}>Clear all</Link><button className="archive-solid" type="submit" disabled={pending}>View results ↗</button></div>
        </form>
      </div></div>
    </section>
    {active.length > 0 && <nav className="archive-active-conditions" aria-label="Applied search conditions">
      {active.map(key => <Link className="archive-condition-chip" key={key} href={without(key)} prefetch={false} aria-label={`Remove ${names[key]}: ${valueLabel(key)}`} onClick={close}><span>{names[key]}: {valueLabel(key)}</span><span aria-hidden="true">×</span></Link>)}
      <Link className="archive-clear" href="/archive#archive-writing" onClick={close}>Clear all</Link>
    </nav>}
    <p className="archive-pending" role="status">{pending ? "Updating articles…" : ""}</p>
  </>;
}
