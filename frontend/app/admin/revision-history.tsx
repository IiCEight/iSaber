"use client";

import { useState } from "react";
import type { Article, Page, Revision } from "../../lib/articles";

type Request = <T>(path: string, init?: RequestInit, signature?: string) => Promise<T>;
type Props = {
  article: Article;
  busy: boolean;
  request: Request;
  run: (action: () => Promise<void>) => Promise<void>;
  onRead: (revision: Revision) => void;
  onRestored: (article: Article) => Promise<void>;
};

function publishedDate(value: string | null) {
  return value ? new Date(value).toLocaleString("en", { timeZone: "Asia/Shanghai" }) : "Not published";
}

export default function RevisionHistory({ article, busy, request, run, onRead, onRestored }: Props) {
  const [versions, setVersions] = useState<Revision[] | null>(null);
  const [next, setNext] = useState<string | null>(null);
  const [selected, setSelected] = useState<Revision | null>(null);

  async function load(cursor?: string) {
    const query = cursor ? `?cursor=${encodeURIComponent(cursor)}` : "";
    const page = await request<Page<Revision>>(`/articles/${article.id}/revisions${query}`);
    setVersions((old) => cursor ? [...(old ?? []), ...page.items] : page.items);
    setNext(page.next_cursor);
  }
  async function read(id: string) {
    const revision = await request<Revision>(`/articles/${article.id}/revisions/${id}`);
    setSelected(revision); onRead(revision);
  }
  async function restore() {
    if (!selected) return;
    const replacement = article.draft ? "This replaces your current draft. " : "";
    if (!confirm(`${replacement}Restore “${selected.title ?? "Untitled"}” from ${publishedDate(selected.first_published_at)} as a new draft? Review and publish it separately to change the live article.`)) return;
    const body = JSON.stringify({ expected_version: article.version });
    const path = `/articles/${article.id}/revisions/${selected.id}/restore`;
    const restored = await request<Article>(path, { method: "POST", headers: { "Content-Type": "application/json" }, body }, `restore:${path}:${body}`);
    await onRestored(restored);
  }

  return <section aria-labelledby="revision-history-title">
    <h3 id="revision-history-title">Published versions</h3>
    <p>Read an earlier version and restore it as a draft. The live article changes only when you publish.</p>
    <button type="button" disabled={busy} onClick={() => run(() => load())}>{versions ? "Refresh versions" : "Load published versions"}</button>
    {versions && (versions.length === 0 ? <p>No published versions yet.</p> : <ul>{versions.map((revision) => <li key={revision.id}>
      <button type="button" disabled={busy} onClick={() => run(() => read(revision.id))}>{revision.title ?? "Untitled"} · {publishedDate(revision.first_published_at)}</button>
      {revision.id === article.published?.id ? " · Current published version" : ""}
    </li>)}</ul>)}
    {next && <button type="button" disabled={busy} onClick={() => run(() => load(next))}>More versions</button>}
    {selected && <p>Selected version: <strong>{selected.title ?? "Untitled"}</strong> · {publishedDate(selected.first_published_at)} <button type="button" disabled={busy} onClick={() => run(restore)}>Restore selected version as draft</button></p>}
  </section>;
}
