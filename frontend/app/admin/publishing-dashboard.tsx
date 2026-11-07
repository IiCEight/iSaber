"use client";

import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import Markdown from "../../components/markdown";
import RevisionHistory from "./revision-history";
import LabelManager from "./label-manager";
import type { AdminLabel, Label, Article, ArticleSummary, ImportJob, Page, Revision } from "../../lib/articles";

class APIError extends Error {
  constructor(public code: string, public status: number) { super(code); }
}

const messages: Record<string, string> = {
  version_conflict: "This article or label changed. Reload it before saving again.",
  invalid_label_name: "Enter a nonblank label name of at most 120 characters.",
  invalid_label: "Check the label name and URL name, then try again.",
  slug_conflict: "That URL name is already used. Choose another one.",
  slug_locked: "The URL stays fixed after first publication.",
  invalid_slug: "Use lowercase letters, numbers, and hyphens for the URL. The name search is reserved.",
  publication_blocked: "Resolve the blocking issues and review the preview before publishing.",
  markdown_or_zip_required: "Choose a Markdown file or a ZIP containing one article and its images.",
  assets_unavailable: "An image file is unavailable. Re-upload the article ZIP before publishing.",
  upload_too_large: "Use up to 5 MiB for Markdown or 50 MiB for a ZIP.",
  request_in_progress: "This request is still being processed. Retry it shortly.",
  idempotency_conflict: "This request key belongs to a different operation. Reload before starting again.",
};

export default function PublishingDashboard({ csrf, onExpired }: { csrf: string; onExpired: () => void }) {
  const [items, setItems] = useState<ArticleSummary[]>([]);
  const [next, setNext] = useState<string | null>(null);
  const [article, setArticle] = useState<Article | null>(null);
  const [labels, setLabels] = useState<{ categories: AdminLabel[]; tags: AdminLabel[] } | null>(null);
  const [preview, setPreview] = useState<Revision | null>(null);
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(true);
  const [message, setMessage] = useState("");
  const [filter, setFilter] = useState({ q: "", status: "", visibility: "" });
  const [uploadTarget, setUploadTarget] = useState<"new" | "selected">("new");
  const [file, setFile] = useState<File | null>(null);
  const [pendingJob, setPendingJob] = useState<ImportJob | null>(null);
  const keys = useRef(new Map<string, string>());
  const listSequence = useRef(0);
  const labelSequence = useRef(0);

  const request = useCallback(async <T,>(path: string, init?: RequestInit, signature?: string): Promise<T> => {
    const headers = new Headers(init?.headers);
    if (init?.method && init.method !== "GET") headers.set("X-CSRF-Token", csrf);
    if (signature) {
      let key = keys.current.get(signature);
      if (!key) { key = crypto.randomUUID(); keys.current.set(signature, key); }
      headers.set("Idempotency-Key", key);
    }
    const response = await fetch(`/api/v1/admin${path}`, { ...init, headers, credentials: "same-origin", cache: "no-store", signal: AbortSignal.timeout(60000) });
    if (!response.ok) {
      const problem = await response.json().catch(() => ({ code: "request_failed" }));
      if (response.status === 401) onExpired();
      if (signature && response.status < 500 && problem.code !== "request_in_progress") keys.current.delete(signature);
      throw new APIError(problem.code ?? "request_failed", response.status);
    }
    const value: T = await response.json();
    if (signature) keys.current.delete(signature);
    return value;
  }, [csrf, onExpired]);

  const fetchLabels = useCallback(async () => {
    const sequence = ++labelSequence.current;
    async function all(kind: "categories" | "tags") {
      const found = new Map<string, AdminLabel>(); let cursor: string | null = null;
      do {
        const query = new URLSearchParams({ limit: "100" }); if (cursor) query.set("cursor", cursor);
        const page: Page<AdminLabel> = await request(`/${kind}?${query}`);
        for (const label of page.items) found.set(label.id, label);
        cursor = page.next_cursor;
      } while (cursor);
      return [...found.values()];
    }
    const [categories, tags] = await Promise.all([all("categories"), all("tags")]);
    return { sequence, value: { categories, tags } };
  }, [request]);
  const loadLabels = useCallback(async () => { const result = await fetchLabels(); if (result.sequence === labelSequence.current) setLabels(result.value); }, [fetchLabels]);
  useEffect(() => {
    let active = true;
    fetchLabels().then((result) => { if (active && result.sequence === labelSequence.current) setLabels(result.value); }).catch(() => { if (active) setMessage("Labels could not be loaded. Use Refresh labels to try again."); });
    return () => { active = false; };
  }, [fetchLabels]);

  const loadList = useCallback(async (cursor?: string) => {
    const sequence = ++listSequence.current;
    const query = new URLSearchParams();
    for (const [key, value] of Object.entries(filter)) if (value.trim()) query.set(key, value.trim());
    if (cursor) query.set("cursor", cursor);
    const page = await request<Page<ArticleSummary>>(`/articles?${query}`);
    if (sequence !== listSequence.current) return;
    setItems((old) => cursor ? [...old, ...page.items] : page.items);
    setNext(page.next_cursor);
  }, [filter, request]);

  useEffect(() => {
    let active = true;
    setLoading(true);
    loadList().catch(() => { if (active) setMessage("Articles could not be loaded. Try reloading the list."); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [loadList]);

  async function selectArticle(id: string) {
    const loaded = await request<Article>(`/articles/${id}`);
    setArticle(loaded); setPreview(null);
    return loaded;
  }
  function explain(error: unknown) {
    setMessage(error instanceof APIError ? (messages[error.code] ?? "The operation could not complete. Check the fields and try again.") : "The result could not be confirmed. Retry the same operation before making changes.");
  }
  async function run(action: () => Promise<void>) {
    setBusy(true); setMessage("");
    try { await action(); } catch (error) { explain(error); } finally { setBusy(false); }
  }
  async function upload(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!pendingJob && (!file || (uploadTarget === "selected" && !article))) return;
    await run(async () => {
      let job = pendingJob;
      if (!job && file) {
      const digest = await crypto.subtle.digest("SHA-256", await file.arrayBuffer());
      const fingerprint = Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, "0")).join("");
      const form = new FormData(); form.append("file", file);
      if (uploadTarget === "selected" && article) { form.append("article_id", article.id); form.append("expected_version", String(article.version)); }
      const signature = `import:${file.name}:${fingerprint}:${uploadTarget === "selected" ? article?.id : "new"}:${uploadTarget === "selected" ? article?.version : ""}`;
      job = await request<ImportJob>("/imports", { method: "POST", body: form }, signature);
      setPendingJob(job);
      }
      if (!job) return;
      for (let attempt = 0; attempt < 240 && (job.status === "queued" || job.status === "processing"); attempt++) {
        setMessage("Importing your article…");
        await new Promise((resolve) => setTimeout(resolve, 500));
        job = await request<ImportJob>(`/imports/${job.id}`);
        setPendingJob(job);
      }
      if (job.status === "failed") { setPendingJob(null); setMessage(job.issues.map((item) => item.message).join(" ")); return; }
      if (job.status !== "succeeded" || !job.article_id) { setMessage("The import is still running. Use Resume import shortly."); return; }
      await selectArticle(job.article_id); await loadList(); setUploadTarget("selected");
      setPendingJob(null);
      setMessage("Draft imported. Review the metadata and preview before publishing.");
    });
  }
  async function saveMetadata(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); if (!article) return;
    const fields = new FormData(event.currentTarget);
    await run(async () => {
      const payload: Record<string, unknown> = { expected_version: article.version, title: fields.get("title"), summary: fields.get("summary") || null, language: fields.get("language"), written_at: fields.get("written_at") || null };
      if (!article.published_at) payload.slug = fields.get("slug");
      payload.category_id = fields.get("category_id") || null; payload.tag_ids = fields.getAll("tag_ids");
      const saved = await request<Article>(`/articles/${article.id}/draft`, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify(payload) });
      setArticle(saved); setPreview(null); await loadList(); setMessage("Draft metadata saved.");
    });
  }
  async function controls(visibility: "public" | "private") {
    if (!article) return;
    await run(async () => {
      if (!confirm(`Make this article ${visibility}? This changes access immediately without publishing the draft.`)) return;
      const saved = await request<Article>(`/articles/${article.id}`, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ expected_version: article.version, visibility }) });
      setArticle(saved); await loadList(); setMessage(`Visibility changed to ${visibility}.`);
    });
  }
  async function publication(action: "publish" | "unpublish") {
    if (!article) return;
    await run(async () => {
      if (!confirm(`${action === "publish" ? `Publish the current draft as ${article.visibility}` : "Unpublish this article"}?`)) return;
      const body = JSON.stringify({ expected_version: article.version });
      await request<Article>(`/articles/${article.id}/${action}`, { method: "POST", headers: { "Content-Type": "application/json" }, body }, `${action}:${article.id}:${body}`);
      await selectArticle(article.id); await loadList();
      setMessage(action === "publish" ? "Article published." : "Article unpublished. Your draft and history are retained.");
    });
  }

  async function labelsChanged() {
    try { await loadLabels(); if (article) await selectArticle(article.id); await loadList(); }
    catch { setMessage("The label was saved. Refresh labels and reload the article before editing further."); return; }
    setMessage("Label saved. Article associations and URL names are preserved.");
  }
  function choices(kind: "categories" | "tags"): Label[] {
    const current = kind === "categories" ? (article?.draft?.category ? [article.draft.category] : []) : (article?.draft?.tags ?? []);
    const found = new Map<string, Label>((labels?.[kind] ?? []).map((label) => [label.id, label]));
    for (const label of current) if (!found.has(label.id)) found.set(label.id, label);
    return [...found.values()];
  }

  return <div className="publishing-dashboard">
    <nav aria-label="Dashboard sections"><a href="#articles">Articles</a> · <a href="#labels">Categories and tags</a></nav>
    <h2 id="articles">Articles</h2>
    {message && <p role="status">{message}</p>}
    <fieldset disabled={busy}>
      <legend>Import article</legend>
      <p>ZIP images: PNG, JPEG, GIF, or WebP, up to 10 MiB and 16 million pixels each. Keep the article’s relative folders inside the ZIP.</p>
      <form onSubmit={upload}>
        <label htmlFor="markdown-file">Markdown (5 MiB) or article ZIP (50 MiB)</label>
        <input id="markdown-file" type="file" accept=".md,.zip,text/markdown,application/zip" required={!pendingJob} disabled={!!pendingJob} onChange={(event) => setFile(event.target.files?.[0] ?? null)} />
        <label htmlFor="upload-target">Import into</label>
        <select id="upload-target" value={uploadTarget} disabled={!!pendingJob} onChange={(event) => setUploadTarget(event.target.value as "new" | "selected")}>
          <option value="new">New private draft</option><option value="selected" disabled={!article}>Selected article, as a draft revision</option>
        </select>
        <button type="submit" disabled={(!file && !pendingJob) || busy}>{pendingJob ? "Resume import" : "Upload article"}</button>
      </form>
    </fieldset>
    <fieldset className="article-filters" disabled={busy}>
      <legend>Filter articles</legend>
      <label>Title search<input value={filter.q} onChange={(event) => setFilter({ ...filter, q: event.target.value })} /></label>
      <label>Status<select value={filter.status} onChange={(event) => setFilter({ ...filter, status: event.target.value })}><option value="">All</option><option value="draft">Draft / unpublished</option><option value="published">Published</option></select></label>
      <label>Visibility<select value={filter.visibility} onChange={(event) => setFilter({ ...filter, visibility: event.target.value })}><option value="">All</option><option value="private">Private</option><option value="public">Public</option></select></label>
      <button type="button" disabled={busy} onClick={() => run(loadList)}>Reload list</button>
    </fieldset>
    {loading ? <p>Loading articles…</p> : items.length === 0 ? <p>No matching articles yet.</p> : <ul>{items.map((item) => <li key={item.id}><button type="button" disabled={busy} onClick={() => run(async () => { await selectArticle(item.id); })}>{item.title || "Untitled draft"}</button> — {item.status}, {item.visibility}{item.has_draft && item.status === "published" ? ", new draft available" : ""}</li>)}</ul>}
    {next && <button type="button" disabled={busy} onClick={() => run(() => loadList(next))}>More articles</button>}
    {article && <section aria-labelledby="selected-article">
      <h2 id="selected-article">{article.title || "Untitled draft"}</h2>
      <p>{article.status} · {article.visibility} · version {article.version}</p>
      {article.published_at && <p>First published: {new Date(article.published_at).toLocaleString("en", { timeZone: "Asia/Shanghai" })}</p>}
      <div className="article-actions">
        <button type="button" disabled={busy} onClick={() => run(async () => { await selectArticle(article.id); })}>Reload article</button>
        <button type="button" disabled={busy} onClick={() => controls(article.visibility === "private" ? "public" : "private")}>Make {article.visibility === "private" ? "public" : "private"}</button>
        {article.published && article.visibility === "public" && <a href={`/articles/${article.slug}`} target="_blank" rel="noreferrer">Read published article</a>}
      </div>
      {article.issues.length > 0 && <ul>{article.issues.map((item) => <li key={`${item.code}-${item.path}-${item.message}`}><strong>{item.severity === "blocking" ? "Required" : "Note"}:</strong> {item.message}{item.path ? ` (${item.path})` : ""}</li>)}</ul>}
      {article.draft && <fieldset disabled={busy || !labels} key={article.draft.id}>
        <legend>Draft metadata</legend>
        <form onSubmit={saveMetadata}>
          <label htmlFor="article-title">Title</label><input id="article-title" name="title" defaultValue={article.draft.title ?? ""} required />
          <label htmlFor="article-slug">URL name</label><input id="article-slug" name="slug" defaultValue={article.slug ?? ""} disabled={!!article.published_at} pattern="[a-z0-9]+(-[a-z0-9]+)*" required />
          <label htmlFor="article-summary">Summary (optional)</label><textarea id="article-summary" name="summary" defaultValue={article.draft.summary ?? ""} />
          <label htmlFor="article-language">Language</label><select id="article-language" name="language" defaultValue={article.draft.language ?? "en"}><option value="en">English</option><option value="zh">中文</option></select>
          <label htmlFor="article-written">Writing date</label><input id="article-written" name="written_at" type="date" defaultValue={article.draft.written_at ?? ""} required />
          <label htmlFor="article-category">Category</label><select id="article-category" name="category_id" defaultValue={article.draft.category?.id ?? ""}>
            <option value="">None</option>{choices("categories").map((label) => <option key={label.id} value={label.id}>{label.name}</option>)}
          </select>
          <fieldset><legend>Tags</legend>{choices("tags").length === 0 ? <p>Create tags in Categories and tags below.</p> : choices("tags").map((label) => <label key={label.id}><input type="checkbox" name="tag_ids" value={label.id} defaultChecked={article.draft?.tags.some((tag) => tag.id === label.id)} /> {label.name}</label>)}</fieldset>
          {!labels && <p>Load labels before editing metadata so saved associations are preserved.</p>}
          <button type="submit">Save metadata</button>
        </form>
      </fieldset>}
      <div className="article-actions">
        <button type="button" disabled={busy || !article.draft} onClick={() => run(async () => { const result = await request<{ revision: Revision }>(`/articles/${article.id}/preview`); setPreview(result.revision); })}>Preview draft</button>
        <button type="button" disabled={busy || !article.published} onClick={() => setPreview(article.published)}>Read current version</button>
        <button type="button" disabled={busy || !article.publishable || !preview || preview.id !== article.draft?.id} onClick={() => publication("publish")}>Publish draft</button>
        <button type="button" disabled={busy || !article.published} onClick={() => publication("unpublish")}>Unpublish</button>
      </div>
      <RevisionHistory key={`${article.id}:${article.published?.id ?? "unpublished"}`} article={article} busy={busy} request={request} run={run} onRead={setPreview} onRestored={async (restored) => {
        setPreview(null);
        try { await selectArticle(restored.id); }
        catch { setMessage("The version was restored. Reload the article to review its latest draft."); return; }
        try { await loadList(); }
        catch { setMessage("The version was restored. Review its draft, then reload the article list."); return; }
        setMessage("Version restored as a draft. Review its metadata and preview before publishing.");
      }} />
      {preview && <section className="article-preview" aria-label={preview.id === article.draft?.id ? "Draft preview" : preview.id === article.published?.id ? "Published article" : "Historical version"}><p><strong>{preview.id === article.draft?.id ? "Draft preview" : preview.id === article.published?.id ? "Published article" : "Historical version"}</strong></p><h2>{preview.title}</h2><p>Written: {preview.written_at ?? "Not set"}{preview.first_published_at ? ` · Version published: ${new Date(preview.first_published_at).toLocaleString("en", { timeZone: "Asia/Shanghai" })}` : ""}{preview.id === article.published?.id && article.updated_at ? ` · Updated: ${new Date(article.updated_at).toLocaleString("en", { timeZone: "Asia/Shanghai" })}` : ""}</p><Markdown content={preview.markdown} /></section>}
    </section>}
    <LabelManager labels={labels} busy={busy} request={request} run={run} onChanged={labelsChanged} refresh={loadLabels} />
  </div>;
}
