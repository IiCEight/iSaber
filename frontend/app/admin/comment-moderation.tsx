"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { AdminBadge, AdminNote } from "./admin-ui";
import { useAdminConfirmation } from "./admin-confirmation";
import type { Article, Page } from "../../lib/articles";
import type { AdminComment, CommentStatus } from "../../lib/comments";

type Props = {
  article: Article | null; busy: boolean; active: boolean;
  request: <T>(path: string, init?: RequestInit) => Promise<T>;
  run: (action: () => Promise<void>) => Promise<void>;
  onOpenArticle: (id: string) => Promise<void>;
};
export default function CommentModeration({ article, busy, active, request, run, onOpenArticle }: Props) {
  const confirmAction = useAdminConfirmation();
  const sequence = useRef(0);
  const [loading, setLoading] = useState(false);
  const [status, setStatus] = useState<CommentStatus | "">("pending");
  const [scope, setScope] = useState("all");
  const [page, setPage] = useState<Page<AdminComment> | null>(null);
  const [message, setMessage] = useState("");
  const articleId = article?.id;
  const load = useCallback(async (cursor?: string) => {
    const current = ++sequence.current;
    const query = new URLSearchParams(); if (status) query.set("status", status); if (scope === "selected" && articleId) query.set("article_id", articleId); if (cursor) query.set("cursor", cursor);
    const result = await request<Page<AdminComment>>(`/comments?${query}`);
    if (current !== sequence.current) return;
    setPage(old => cursor ? { ...result, items: [...(old?.items ?? []), ...result.items] } : result);
  }, [status, scope, articleId, request]);
  useEffect(() => {
    if (!active) return;
    let current = true;
    setLoading(true); setPage(null); setMessage("");
    load().catch(() => { if (current) setMessage("Comments could not be loaded. Use Reload comments to try again."); }).finally(() => { if (current) setLoading(false); });
    return () => { current = false; sequence.current++; };
  }, [active, load]);
  async function moderate(comment: AdminComment, next: Exclude<CommentStatus, "pending">) {
    const effect = next === "approved" ? "make it visible while its article is public and published" : "hide it from public comments";
    if (!await confirmAction(`${next === "approved" ? "Approve" : next === "rejected" ? "Reject" : "Remove"} the comment by “${comment.display_name}” and ${effect}?`)) return;
    await request<AdminComment>(`/comments/${comment.id}`, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ status: next, expected_version: comment.version }) });
    try { await load(); setMessage("Moderation saved."); } catch { setMessage("Moderation saved. Reload comments to review the updated list."); }
  }
  return <section id="comments" aria-labelledby="moderation-title">
    <section className="card"><div className="card-head"><div><h2 id="moderation-title">Review queue</h2><p>Plain-text comments, reviewed by the owner.</p></div></div>
    {message && <p className="admin-inline-note" role="status">{message}</p>}
    <fieldset disabled={busy || loading}><legend className="sr-only">Comment filters</legend><form className="toolbar" onSubmit={event => { event.preventDefault(); void run(async () => { setMessage(""); await load(); }); }}>
      <label htmlFor="comment-status">Status<select id="comment-status" value={status} onChange={event => setStatus(event.target.value as CommentStatus | "")}><option value="">All statuses</option>{(["pending", "approved", "rejected", "removed"] as const).map(value => <option key={value} value={value}>{value[0].toUpperCase() + value.slice(1)}</option>)}</select></label>
      <label htmlFor="comment-scope">Article<select id="comment-scope" value={scope} onChange={event => setScope(event.target.value)}><option value="all">All articles</option><option value="selected" disabled={!article}>Selected article{article ? `: ${article.title ?? "Untitled"}` : ""}</option></select></label>
      <button className="button" type="submit">Reload comments</button>
    </form></fieldset>
    {loading ? <p className="empty" role="status">Loading comments…</p> : page ? page.items.length === 0 ? <div className="empty"><h3>{status === "pending" ? "All caught up" : "No matching comments"}</h3><p>{status === "pending" ? "No comments are waiting for review in this view." : "Choose another status or article to continue."}</p></div> : <ol className="comment-list">{page.items.map(comment => <li className="comment-item" key={comment.id}>
      <article aria-label={`Comment by ${comment.display_name}`}><div className="comment-heading"><span className="avatar" aria-hidden="true">{Array.from(comment.display_name)[0]}</span><div><strong>{comment.display_name}</strong><p><time dateTime={comment.created_at}>{new Date(comment.created_at).toLocaleString("en", { timeZone: "Asia/Shanghai" })}</time></p></div><AdminBadge tone={comment.status === "pending" ? "amber" : comment.status === "approved" ? "green" : ""}>{comment.status[0].toUpperCase() + comment.status.slice(1)}</AdminBadge></div>
      <p className="comment-body">{comment.body}</p><div className="comment-actions">{(["approved", "rejected", "removed"] as const).map(next => <button className={`button small ${next === "approved" ? "primary" : next === "removed" ? "danger" : ""}`} key={next} type="button" disabled={busy || comment.status === next} onClick={() => run(() => moderate(comment, next))}>{next === "approved" ? "Approve" : next === "rejected" ? "Reject" : "Remove"}</button>)}<button className="row-action" type="button" disabled={busy} onClick={() => run(() => onOpenArticle(comment.article_id))}>Open associated article →</button></div></article>
    </li>)}</ol> : <p className="empty">Reload comments to review submissions.</p>}
    <div className="table-foot"><span>{page ? `${page.items.length} comments loaded` : "Review queue"}</span>{page?.next_cursor && <button className="button small" type="button" disabled={busy || loading} onClick={() => run(() => load(page.next_cursor ?? undefined))}>More submissions</button>}</div>
    </section><AdminNote>Approval makes a comment readable only while its article is public and published. Removed comments remain retained for owner review.</AdminNote>
  </section>;
}
