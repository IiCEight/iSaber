"use client";

import { useEffect, useRef, useState, type FormEvent } from "react";
import type { Page } from "../lib/articles";
import type { PublicComment } from "../lib/comments";

type Attempt = { hash: string; key: string };
export default function ArticleComments({ articleId, enabled, initialPage, unavailable }: {
  articleId: string; enabled: boolean; initialPage: Page<PublicComment> | null; unavailable: boolean;
}) {
  const [page, setPage] = useState(initialPage);
  const [active, setActive] = useState(true);
  const [ready, setReady] = useState(false);
  useEffect(() => { setReady(true); }, []);
  const [canSubmit, setCanSubmit] = useState(enabled);
  const [name, setName] = useState(""); const [body, setBody] = useState("");
  const [busy, setBusy] = useState(false); const [uncertain, setUncertain] = useState(false);
  const [wait, setWait] = useState(0);
  const [message, setMessage] = useState(unavailable ? "Comments are temporarily unavailable. Try refreshing comments." : "");
  const attempt = useRef<Attempt | null>(null);
  const storageKey = `isaber-comment-attempt:${articleId}`;
  useEffect(() => { if (wait <= 0) return; const timer = setTimeout(() => setWait(wait - 1), 1000); return () => clearTimeout(timer); }, [wait]);

  async function load(more = false) {
    setBusy(true);
    try {
      const query = more && page?.next_cursor ? `?cursor=${encodeURIComponent(page.next_cursor)}` : "";
      const response = await fetch(`/api/v1/articles/${articleId}/comments${query}`, { cache: "no-store", credentials: "omit", signal: AbortSignal.timeout(10000) });
      if (response.status === 404) { setActive(false); setCanSubmit(false); setPage(null); setMessage("Comments are no longer available for this article."); return; }
      if (!response.ok) throw new Error("unavailable");
      const next: Page<PublicComment> = await response.json();
      setPage((old) => more ? { ...next, items: [...(old?.items ?? []), ...next.items] } : next);
      setMessage("Comments refreshed.");
    } catch { setMessage("Comments could not be loaded. Your entered text is kept."); } finally { setBusy(false); }
  }
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setBusy(true); setMessage("");
    try {
      const payload = JSON.stringify({ display_name: name, body });
      const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(payload));
      const hash = Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, "0")).join("");
      if (!attempt.current) { try { const saved = sessionStorage.getItem(storageKey); if (saved) {
        const value: unknown = JSON.parse(saved);
        if (value && typeof value === "object" && "hash" in value && "key" in value && typeof value.hash === "string" && /^[0-9a-f]{64}$/.test(value.hash) && typeof value.key === "string" && /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(value.key)) attempt.current = { hash: value.hash, key: value.key };
      } } catch { /* Retry state can fall back to page memory. */ } }
      if (attempt.current?.hash !== hash) attempt.current = { hash, key: crypto.randomUUID() };
      try { sessionStorage.setItem(storageKey, JSON.stringify(attempt.current)); } catch { /* Private/limited storage may be unavailable. */ }
      const response = await fetch(`/api/v1/articles/${articleId}/comments`, { method: "POST", credentials: "omit", headers: { "Content-Type": "application/json", "Idempotency-Key": attempt.current.key }, body: payload, cache: "no-store", signal: AbortSignal.timeout(10000) });
      const result = await response.json();
      if (response.status === 202 && result.status === "pending") {
        attempt.current = null; try { sessionStorage.removeItem(storageKey); } catch { /* Page-memory state has already been cleared. */ }
        setBody(""); setUncertain(false); setMessage("Comment received. It will appear only after owner approval."); return;
      }
      if (response.status >= 500 || result.code === "request_in_progress" || result.code === "idempotency_conflict") setUncertain(true);
      if (response.status === 404) { setActive(false); setCanSubmit(false); setPage(null); }
      if (result.code === "comments_disabled") setCanSubmit(false);
      if (response.status === 429) setWait(Math.min(600, Math.max(1, Number(response.headers.get("Retry-After")) || 60)));
      const explanations: Record<string, string> = {
        idempotency_conflict: "The saved retry key belongs to a different submission. Use Edit as a new comment to start over; your text is kept.",
        invalid_comment: "Enter a nonblank name (up to 80 characters) and comment (up to 5,000 characters). Your text is kept.",
        request_too_large: "The submission is too large. Shorten the name or comment; your text is kept.",
        comments_disabled: "New comments are disabled. Your entered text is kept.",
        comment_rate_limited: "Too many recent submissions. Wait before retrying; your text is kept.",
        not_found: "Comments are no longer available for this article. Your entered text is kept.",
        invalid_origin: "This page address could not be verified. Open the blog at its configured address before retrying.",
        request_in_progress: "Submission is still being processed. Retry the same submission shortly.",
      };
      setMessage(explanations[result.code] ?? "Submission could not be confirmed. Retry the same submission before starting another.");
    } catch { setUncertain(true); setMessage("Submission could not be confirmed. Your text is kept; retry the same submission."); }
    finally { setBusy(false); }
  }
  function editNew() {
    if (!confirm("The previous submission may already be awaiting approval. Discard its retry state and edit a new comment?")) return;
    attempt.current = null; try { sessionStorage.removeItem(storageKey); } catch { /* Local-only state cleanup. */ }
    setUncertain(false); setMessage("Retry state cleared. Your text is kept for editing as a new comment.");
  }
  return <section className="article-comments" aria-labelledby="comments-title">
    <h2 id="comments-title">Comments</h2>
    {message && <p role="status">{message}</p>}
    {active && <>
      <button type="button" disabled={busy || !ready} onClick={() => load()}>Refresh comments</button>
      {page && (page.items.length === 0 ? <p>No approved comments yet.</p> : <ol>{page.items.map((comment) => <li key={comment.id}>
        <p><strong>{comment.display_name}</strong> · <time dateTime={comment.created_at}>{new Date(comment.created_at).toLocaleString("en", { timeZone: "Asia/Shanghai" })}</time></p>
        <p className="comment-body">{comment.body}</p>
      </li>)}</ol>)}
      {page?.next_cursor && <button type="button" disabled={busy || !ready} onClick={() => load(true)}>More comments</button>}
    </>}
    {!canSubmit && <p>New comments are disabled or unavailable. Approved comments remain readable while the article is public.</p>}
    <p>Your display name and plain-text comment become public only after owner approval. No account or email is required.</p>
    <noscript>Enable JavaScript to submit a comment. Approved comments remain readable.</noscript>
    <form method="post" action={`/api/v1/articles/${articleId}/comments`} onSubmit={submit}>
      <label htmlFor="comment-name">Display name</label><input id="comment-name" name="display_name" value={name} onChange={(event) => setName(event.target.value)} maxLength={80} required disabled={busy || !ready || uncertain || !canSubmit} />
      <label htmlFor="comment-body">Comment</label><textarea id="comment-body" name="body" value={body} onChange={(event) => setBody(event.target.value)} maxLength={5000} required disabled={busy || !ready || uncertain || !canSubmit} />
      <button type="submit" disabled={busy || !ready || !canSubmit || wait > 0}>{busy ? "Submitting…" : wait > 0 ? `Retry in ${wait}s` : uncertain ? "Retry same submission" : "Submit for moderation"}</button>
    </form>
    {uncertain && <button type="button" disabled={busy} onClick={editNew}>Edit as a new comment</button>}
  </section>;
}
