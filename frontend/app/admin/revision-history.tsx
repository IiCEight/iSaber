"use client";

import { AdminBadge, AdminTable, adminDateTime } from "./admin-ui";
import { useAdminConfirmation } from "./admin-confirmation";
import { useCallback, useEffect, useRef, useState } from "react";
import type { Article, Page, Revision } from "../../lib/articles";

type Request = <T>(path: string, init?: RequestInit, signature?: string) => Promise<T>;
type Props = {
  article: Article;
  active: boolean;
  busy: boolean;
  request: Request;
  run: (action: () => Promise<void>) => Promise<void>;
  onRead: (revision: Revision) => void;
  onRestored: (article: Article) => Promise<void>;
};

export default function RevisionHistory({ article, active, busy, request, run, onRead, onRestored }: Props) {
  const confirmAction = useAdminConfirmation();
  const [versions, setVersions] = useState<Revision[] | null>(null);
  const [next, setNext] = useState<string | null>(null);
  const [selected, setSelected] = useState<Revision | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const sequence = useRef(0);

  const load = useCallback(async (cursor?: string) => {
    const current = ++sequence.current;
    setLoading(true); setError("");
    const query = cursor ? `?cursor=${encodeURIComponent(cursor)}` : "";
    try {
      const page = await request<Page<Revision>>(`/articles/${article.id}/revisions${query}`);
      if (current !== sequence.current) return;
      setVersions((old) => cursor ? [...new Map([...(old ?? []), ...page.items].map(revision => [revision.id, revision])).values()] : page.items);
      setNext(page.next_cursor);
    } catch {
      if (current === sequence.current) setError("Published versions could not be loaded. Try again.");
    } finally {
      if (current === sequence.current) setLoading(false);
    }
  }, [article.id, request]);

  useEffect(() => {
    if (!active) return;
    void load();
    return () => { sequence.current++; };
  }, [active, load]);
  async function read(id: string) {
    const revision = await request<Revision>(`/articles/${article.id}/revisions/${id}`);
    setSelected(revision); onRead(revision);
  }
  async function restore() {
    if (!selected) return;
    const replacement = article.draft ? "This replaces your current draft. " : "";
    if (!await confirmAction(`${replacement}Restore “${selected.title ?? "Untitled"}” from ${adminDateTime(selected.first_published_at)} as a new draft? Review and publish it separately to change the live article.`)) return;
    const body = JSON.stringify({ expected_version: article.version });
    const path = `/articles/${article.id}/revisions/${selected.id}/restore`;
    const restored = await request<Article>(path, { method: "POST", headers: { "Content-Type": "application/json" }, body }, `restore:${path}:${body}`);
    await onRestored(restored);
  }

  return <section aria-labelledby="revision-history-title">
    <div className="card-body admin-history-heading"><div><h2 id="revision-history-title">Published versions</h2><p className="help">Read an earlier version and restore it as a draft. The live article changes only when you publish.</p></div><button className="button small" type="button" disabled={busy || loading} onClick={() => void load()}>Refresh versions</button></div>
    {error && <div className="admin-inline-note"><p role="alert">{error}</p><button className="button" type="button" disabled={busy || loading} onClick={() => void load()}>Retry versions</button></div>}
    {loading ? <p className="card-body muted" role="status">Loading published versions…</p> : !error && versions && (versions.length === 0 ? <div className="empty"><h3>No published versions yet</h3><p>Versions appear after the first publication.</p></div> : <AdminTable label="Published versions"><table className="admin-version-table"><thead><tr><th scope="col">Version</th><th scope="col">Published</th><th scope="col">Actions</th></tr></thead><tbody>{versions.map(revision => <tr key={revision.id}><td><strong>{revision.title ?? "Untitled"}</strong>{revision.id === article.published?.id && <div className="help"><AdminBadge tone="blue">Current published version</AdminBadge></div>}</td><td className="date-cell">{adminDateTime(revision.first_published_at)}</td><td><button className="row-action" type="button" disabled={busy || loading} onClick={() => run(() => read(revision.id))}>Read version</button></td></tr>)}</tbody></table></AdminTable>)}
    {next && !error && <div className="card-body"><button className="button" type="button" disabled={busy || loading} onClick={() => void load(next)}>More versions</button></div>}
    {selected && <div className="admin-history-selection"><p>Selected: <strong>{selected.title ?? "Untitled"}</strong> · {adminDateTime(selected.first_published_at)}</p><button className="button" type="button" disabled={busy} onClick={() => run(restore)}>Restore selected version as draft</button></div>}
  </section>;
}
