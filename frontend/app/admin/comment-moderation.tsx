"use client";

import { useEffect, useState } from "react";
import type { Article, Page } from "../../lib/articles";
import type { AdminComment, CommentStatus } from "../../lib/comments";

type Props = {
  article: Article | null; busy: boolean;
  request: <T>(path: string, init?: RequestInit) => Promise<T>;
  run: (action: () => Promise<void>) => Promise<void>;
  onOpenArticle: (id: string) => Promise<void>;
};
export default function CommentModeration({ article, busy, request, run, onOpenArticle }: Props) {
  const [status, setStatus] = useState<CommentStatus | "">("pending");
  const [scope, setScope] = useState("all");
  const [page, setPage] = useState<Page<AdminComment> | null>(null);
  const [message, setMessage] = useState("");
  useEffect(() => { if (scope === "selected" && article?.id) setPage(null); }, [article?.id, scope]);
  async function load(cursor?: string) {
    const query = new URLSearchParams(); if (status) query.set("status", status); if (scope === "selected" && article) query.set("article_id", article.id); if (cursor) query.set("cursor", cursor);
    const result = await request<Page<AdminComment>>(`/comments?${query}`);
    setPage((old) => cursor ? { ...result, items: [...(old?.items ?? []), ...result.items] } : result);
  }
  async function moderate(comment: AdminComment, next: Exclude<CommentStatus, "pending">) {
    const effect = next === "approved" ? "make it visible while its article is public and published" : "hide it from public comments";
    if (!confirm(`${next === "approved" ? "Approve" : next === "rejected" ? "Reject" : "Remove"} the comment by “${comment.display_name}” and ${effect}?`)) return;
    await request<AdminComment>(`/comments/${comment.id}`, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ status: next, expected_version: comment.version }) });
    try { await load(); setMessage("Moderation saved."); } catch { setMessage("Moderation saved. Reload comments to review the updated list."); }
  }
  return <section id="comments" aria-labelledby="moderation-title">
    <h2 id="moderation-title">Comment moderation</h2>
    <p>Review plain-text submissions before approval. Removed comments remain retained for owner review.</p>
    {message && <p role="status">{message}</p>}
    <fieldset className="article-filters" disabled={busy}>
      <legend>Comment filters</legend>
      <label>Status<select value={status} onChange={(event) => { setStatus(event.target.value as CommentStatus | ""); setPage(null); }}><option value="">All</option>{(["pending", "approved", "rejected", "removed"] as const).map((value) => <option key={value} value={value}>{value}</option>)}</select></label>
      <label>Article<select value={scope} onChange={(event) => { setScope(event.target.value); setPage(null); }}><option value="all">All articles</option><option value="selected" disabled={!article}>Selected article{article ? `: ${article.title ?? "Untitled"}` : ""}</option></select></label>
      <button type="button" disabled={busy} onClick={() => run(async () => { setMessage(""); await load(); })}>Load comments</button>
    </fieldset>
    {page && (page.items.length === 0 ? <p>No comments match these filters.</p> : <ol>{page.items.map((comment) => <li key={comment.id}>
      <p><strong>{comment.display_name}</strong> · {comment.status} · <time dateTime={comment.created_at}>{new Date(comment.created_at).toLocaleString("en", { timeZone: "Asia/Shanghai" })}</time></p>
      <p className="comment-body">{comment.body}</p>
      <div className="article-actions">
        <button type="button" disabled={busy} onClick={() => run(() => onOpenArticle(comment.article_id))}>Open associated article</button>
        {(["approved", "rejected", "removed"] as const).map((next) => <button key={next} type="button" disabled={busy || comment.status === next} onClick={() => run(() => moderate(comment, next))}>{next === "approved" ? "Approve" : next === "rejected" ? "Reject" : "Remove"}</button>)}
      </div>
    </li>)}</ol>)}
    {page?.next_cursor && <button type="button" disabled={busy} onClick={() => run(() => load(page.next_cursor ?? undefined))}>More submissions</button>}
  </section>;
}
