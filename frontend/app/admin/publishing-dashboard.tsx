"use client";

import { useCallback, useEffect, useRef, useState, type FormEvent, type DragEvent } from "react";
import Markdown from "../../components/markdown";
import RevisionHistory from "./revision-history";
import AdminPagination from "./admin-pagination";
import ArticleMetadata from "./article-metadata";
import LabelManager from "./label-manager";
import CommentModeration from "./comment-moderation";
import { AdminBadge, AdminIcon, AdminNote, AdminTable, adminDate } from "./admin-ui";
import { useAdminConfirmation } from "./admin-confirmation";
import { articleOutline } from "../../lib/article-outline";
import { updatedAtInput, updatedAtTimestamp } from "../../lib/article-dates";
import type { AdminLabel, Label, Article, ArticleSummary, ImportJob, NumberedPage, Page, Revision } from "../../lib/articles";

class APIError extends Error {
  constructor(public code: string, public status: number) { super(code); }
}

const messages: Record<string, string> = {
  version_conflict: "This article, label, or comment changed. Reload it before saving again.",
  invalid_label_name: "Enter a nonblank label name of at most 120 characters.",
  invalid_label: "Check the label name and URL name, then try again.",
  slug_conflict: "That URL name is already used. Choose another one.",
  slug_locked: "The URL stays fixed after first publication.",
  invalid_slug: "Use lowercase letters, numbers, and hyphens for the URL. The name search is reserved.",
  invalid_updated_at: "Enter a valid last updated time, or leave it blank to use publication time.",
  publication_blocked: "Resolve the blocking issues and review the preview before publishing.",
  markdown_or_zip_required: "Choose a Markdown file or a ZIP containing one article and its images.",
  assets_unavailable: "An image file is unavailable. Re-upload the article ZIP before publishing.",
  upload_too_large: "Use up to 5 MiB for Markdown or 50 MiB for a ZIP.",
  request_in_progress: "This request is still being processed. Retry it shortly.",
  idempotency_conflict: "This request key belongs to a different operation. Reload before starting again.",
};

type View = "articles" | "import" | "editor" | "labels" | "comments";
export default function PublishingDashboard({ csrf, onExpired, username, signingOut, onSignOut, sessionMessage }: {
  csrf: string; onExpired: () => void; username: string; signingOut: boolean; onSignOut: () => Promise<void>; sessionMessage: string;
}) {
  const confirmAction = useAdminConfirmation();
  const [view, setView] = useState<View>("articles");
  const [editorTab, setEditorTab] = useState<"metadata" | "preview" | "history">("metadata");
  const [dirty, setDirty] = useState(false);
  const [ready, setReady] = useState(false);
  const [listError, setListError] = useState("");
  const [filterFields, setFilterFields] = useState({ q: "", status: "", visibility: "" });
  const workspace = useRef<HTMLElement>(null);
  const metadataForm = useRef<HTMLFormElement>(null);
  const fileInput = useRef<HTMLInputElement>(null);
  useEffect(() => { setReady(true); }, []);
  useEffect(() => {
    if (!dirty) return;
    const warn = (event: BeforeUnloadEvent) => { event.preventDefault(); event.returnValue = ""; };
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [dirty]);
  const [collection, setCollection] = useState<NumberedPage<ArticleSummary> | null>(null);
  const [pageNumber, setPageNumber] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const items = collection?.items ?? [];
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
  const locked = busy || signingOut || !ready;
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

  const loadList = useCallback(async () => {
    const sequence = ++listSequence.current;
    setLoading(true); setListError("");
    const query = new URLSearchParams({ page: String(pageNumber), limit: String(pageSize) });
    for (const [key, value] of Object.entries(filter)) if (value.trim()) query.set(key, value.trim());
    try {
      const result = await request<NumberedPage<ArticleSummary>>(`/articles?${query}`);
      if (sequence === listSequence.current) setCollection(result);
    } catch (error) {
      if (sequence === listSequence.current) setListError("Articles could not be loaded. Reload the list to try again.");
      throw error;
    } finally {
      if (sequence === listSequence.current) setLoading(false);
    }
  }, [filter, pageNumber, pageSize, request]);

  useEffect(() => {
    void loadList().catch(() => { /* The collection displays its own retry state. */ });
    return () => { listSequence.current++; };
  }, [loadList]);

  function applyFilters(applied: typeof filter) {
    setFilter(applied); setPageNumber(1);
  }
  function changePage(number: number) {
    setPageNumber(number);
    requestAnimationFrame(() => { workspace.current?.focus(); workspace.current?.scrollIntoView({ block: "start", behavior: "instant" }); });
  }

  async function selectArticle(id: string) {
    const loaded = await request<Article>(`/articles/${id}`);
    setArticle(loaded); setPreview(loaded.published); setDirty(false);
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
      await selectArticle(job.article_id); setUploadTarget("selected");
      setPendingJob(null); setFile(null); if (fileInput.current) fileInput.current.value = "";
      setView("editor"); setEditorTab("metadata");
      requestAnimationFrame(() => workspace.current?.focus());
      setMessage("Draft imported. Review the metadata and preview before publishing.");
      try { await loadList(); } catch { setMessage("Draft imported. The article list could not refresh; reload the list when ready."); }
    });
  }
  async function saveMetadata(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); if (!article) return;
    const fields = new FormData(event.currentTarget);
    await run(async () => {
      const payload: Record<string, unknown> = { expected_version: article.version, title: fields.get("title"), summary: fields.get("summary") || null, language: fields.get("language"), written_at: fields.get("written_at") || null };
      payload.updated_at = updatedAtTimestamp(String(fields.get("updated_at") || ""));
      if (!article.published_at) payload.slug = fields.get("slug");
      payload.category_id = fields.get("category_id") || null; payload.tag_ids = fields.getAll("tag_ids");
      const saved = await request<Article>(`/articles/${article.id}/draft`, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify(payload) });
      setArticle(saved); setPreview(null); setDirty(false); setMessage("Draft metadata saved.");
      try { await loadList(); } catch { setMessage("Draft metadata saved. Reload the article list to refresh it."); }
    });
    requestAnimationFrame(() => metadataForm.current?.querySelector<HTMLButtonElement>("button[type=submit]")?.focus());
  }
  async function controls(visibility: "public" | "private") {
    if (!article) return;
    await run(async () => {
      if (!await confirmAction(`Make this article ${visibility}? This changes access immediately without publishing the draft.`)) return;
      const saved = await request<Article>(`/articles/${article.id}`, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ expected_version: article.version, visibility }) });
      setArticle(saved); await loadList(); setMessage(`Visibility changed to ${visibility}.`);
    });
  }
  async function publication(action: "publish" | "unpublish") {
    if (!article) return;
    await run(async () => {
      if (!await confirmAction(`${action === "publish" ? `Publish the current draft as ${article.visibility}` : "Unpublish this article"}?`)) return;
      const body = JSON.stringify({ expected_version: article.version });
      const saved = await request<Article>(`/articles/${article.id}/${action}`, { method: "POST", headers: { "Content-Type": "application/json" }, body }, `${action}:${article.id}:${body}`);
      setArticle(saved); setPreview(null); setDirty(false); setEditorTab("metadata");
      const result = action === "publish" ? "Article published." : "Article unpublished. Your draft and history are retained.";
      setMessage(result);
      try { await loadList(); } catch { setMessage(`${result} Reload the article list to refresh it.`); }
    });
  }

  async function toggleComments() {
    if (!article) return;
    await run(async () => {
      if (!await confirmAction(`${article.comments_enabled ? "Disable" : "Enable"} new comments on “${article.title ?? "Untitled"}”? Existing approved comments stay readable while the article is public.`)) return;
      const saved = await request<Article>(`/articles/${article.id}`, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ expected_version: article.version, comments_enabled: !article.comments_enabled }) });
      setArticle(saved); await loadList(); setMessage(`New comments ${saved.comments_enabled ? "enabled" : "disabled"}.`);
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

  async function discardChanges() {
    if (!dirty) return true;
    if (!await confirmAction("The current metadata changes have not been saved. Discard them and continue?", { title: "Leave unsaved changes?", action: "Discard changes", danger: true })) return false;
    metadataForm.current?.reset(); setDirty(false); return true;
  }
  async function navigate(nextView: View) {
    if (locked || (view === "editor" && nextView !== "editor" && !await discardChanges())) return;
    setView(nextView); setMessage("");
    requestAnimationFrame(() => { workspace.current?.focus(); window.scrollTo({ top: 0, behavior: "instant" }); });
  }
  async function openArticle(id: string) {
    if (!await discardChanges()) return;
    await selectArticle(id); setView("editor"); setEditorTab("metadata");
    requestAnimationFrame(() => workspace.current?.focus());
  }
  function chooseFile(value: File | null) {
    if (!value) { setFile(null); return; }
    const zip = /\.zip$/i.test(value.name);
    if ((!zip && !/\.md$/i.test(value.name)) || value.size > (zip ? 50 : 5) * 1024 * 1024) {
      setFile(null); if (fileInput.current) fileInput.current.value = "";
      setMessage("Choose a Markdown file up to 5 MiB or an article ZIP up to 50 MiB."); return;
    }
    setFile(value); setMessage("");
  }
  function dropFile(event: DragEvent<HTMLDivElement>) {
    event.preventDefault(); if (!locked && !pendingJob) chooseFile(event.dataTransfer.files[0] ?? null);
  }
  async function reviewDraft() {
    if (!article || dirty) return;
    const result = await request<{ revision: Revision; article_version: number }>(`/articles/${article.id}/preview`);
    if (result.article_version !== article.version) {
      setPreview(null); setMessage("This article changed. Reload it before reviewing and publishing."); return;
    }
    setPreview(result.revision); setEditorTab("preview");
    requestAnimationFrame(() => document.getElementById("admin-preview-content")?.focus());
  }
  const canPublish = !!article?.draft && article.publishable && preview?.id === article.draft.id && !dirty;
  const viewName = { articles: "Articles", import: "Import article", editor: "Article details", labels: "Categories & tags", comments: "Comments" }[view];
  return <div className="shell">
    <aside className="sidebar" aria-label="Workspace sidebar">
      <a className="brand" href="/" target="_blank" rel="noreferrer"><span className="brand-mark" aria-hidden="true">iS</span><span>iSaber<small>Content workspace</small></span></a>
      <p className="nav-heading">MANAGE</p>
      <nav aria-label="Workspace">{([ ["articles", "Articles"], ["import", "Import article"], ["labels", "Categories & tags"], ["comments", "Comments"] ] as const).map(([key, name]) => <button key={key} type="button" disabled={locked} aria-current={view === key || (key === "articles" && view === "editor") ? "page" : undefined} onClick={() => { if (key === "import") setUploadTarget("new"); void navigate(key); }}><AdminIcon name={key} /><span>{name}</span></button>)}</nav>
      <div className="sidebar-foot"><span className="avatar" aria-hidden="true">{username.slice(0, 1).toUpperCase()}</span><div><strong>{username}</strong><small>Owner workspace</small></div></div>
    </aside>
    <div className="workspace">
      <header className="topbar"><span>Workspace / {viewName}</span><div className="topbar-actions"><a className="text-link" href="/" target="_blank" rel="noreferrer">View blog ↗</a><button className="text-link" type="button" disabled={locked} onClick={async () => { if (await discardChanges()) await onSignOut(); }}>{signingOut ? "Signing out…" : "Sign out"}</button></div></header>
      <section className="admin-content" ref={workspace} tabIndex={-1} aria-label={viewName}>
        <noscript>Enable JavaScript to manage articles, imports, labels, and comments.</noscript>
        {sessionMessage && <p className="admin-alert" role="alert">{sessionMessage}</p>}
        <div className="admin-message" role="status">{message}</div>
        <div hidden={view !== "articles"}>
          <div className="page-heading"><div><h1>Articles</h1><p>Manage drafts, published writing, and article visibility.</p></div><button className="button primary" type="button" disabled={locked} onClick={() => { setUploadTarget("new"); void navigate("import"); }}><AdminIcon name="import" />Import article</button></div>
          <section className="card" aria-label="Article collection">
            <fieldset className="list-tabs" aria-label="Article status">{([ ["", "All articles"], ["published", "Published"], ["draft", "Drafts"] ] as const).map(([value, label]) => <button type="button" key={value} disabled={locked} aria-pressed={filter.status === value} onClick={() => { const applied = { ...filter, status: value }; applyFilters(applied); setFilterFields(applied); }}>{label}</button>)}</fieldset>
            <form className="toolbar" onSubmit={event => { event.preventDefault(); applyFilters({ ...filterFields }); }}>
              <label className="search-label">Search articles<input type="search" value={filterFields.q} placeholder="Search by title…" disabled={locked} onChange={event => setFilterFields({ ...filterFields, q: event.target.value })} /></label>
              <label>Visibility<select value={filterFields.visibility} disabled={locked} onChange={event => setFilterFields({ ...filterFields, visibility: event.target.value })}><option value="">All visibility</option><option value="public">Public</option><option value="private">Private</option></select></label>
              <button className="button" type="submit" disabled={locked}>Apply filters</button><button type="button" className="reset" disabled={locked} onClick={() => { const clean = { q: "", status: "", visibility: "" }; setFilterFields(clean); applyFilters(clean); }}>Reset</button>
            </form>
            {loading ? <p className="empty" role="status">Loading articles…</p> : listError ? <div className="empty"><h2>The article list is unavailable</h2><p>{listError}</p><button className="button" type="button" disabled={locked} onClick={() => run(loadList)}>Reload list</button></div> : items.length === 0 ? <div className="empty"><div className="empty-icon"><AdminIcon name="articles" /></div><h2>No matching articles yet</h2><p>Adjust your filters or import your first Markdown article.</p></div> : <AdminTable label="Articles table"><table><thead><tr><th scope="col">Article</th><th scope="col">Status</th><th scope="col">Visibility</th><th scope="col">Modified</th><th scope="col">Actions</th></tr></thead><tbody>{items.map(item => <tr key={item.id}><td className="article-cell"><button className="article-title" type="button" disabled={locked} onClick={() => run(() => openArticle(item.id))}>{item.title || "Untitled draft"}</button><div className="row-meta"><span>{item.slug ? `/articles/${item.slug}` : "URL not set"}</span>{item.has_draft && item.status === "published" && <span>Unpublished changes</span>}</div></td><td><AdminBadge tone={item.status === "published" ? "green" : "amber"}>{item.status === "published" ? "Published" : "Draft"}</AdminBadge></td><td><AdminBadge tone={item.visibility === "public" ? "blue" : ""}>{item.visibility === "public" ? "Public" : "Private"}</AdminBadge></td><td className="date-cell"><time dateTime={item.modified_at}>{adminDate(item.modified_at)}</time></td><td><button className="row-action" type="button" disabled={locked} aria-label={`Edit ${item.title || "untitled draft"}`} onClick={() => run(() => openArticle(item.id))}>Edit</button></td></tr>)}</tbody></table></AdminTable>}
            <div className="table-foot admin-article-pagination">
              <div className="admin-pagination-summary"><span role="status">{loading ? "Loading articles…" : listError ? "List unavailable" : collection?.total ? `${(collection.page - 1) * collection.page_size + 1}–${(collection.page - 1) * collection.page_size + items.length} of ${collection.total} articles` : "0 articles"}</span>
                <label className="admin-page-size" htmlFor="admin-page-size">Articles per page<select id="admin-page-size" value={pageSize} disabled={locked || loading} onChange={event => { setPageSize(Number(event.target.value)); setPageNumber(1); }}>{[5, 10, 20].map(size => <option key={size} value={size}>{size}</option>)}</select></label>
              </div>
              {collection && !listError && <AdminPagination current={collection.page} total={collection.total_pages} disabled={locked || loading} onChange={changePage} />}
            </div>
          </section>
          <AdminNote><strong>Write in Typora, manage it here.</strong> New Markdown and ZIP imports start as private drafts.</AdminNote>
        </div>
        <div hidden={view !== "import"}>
          <div className="page-heading"><div><h1>Import article</h1><p>Bring your writing from Typora. Review the draft before publishing.</p></div></div>
          <div className="import-grid"><section className="card"><div className="card-head"><div><h2>Upload your file</h2><p>One Markdown article, with or without its images.</p></div></div><fieldset className="card-body fields" disabled={locked}>
            <form onSubmit={upload} className="fields">
              <label htmlFor="upload-target">Import into<select id="upload-target" value={uploadTarget} disabled={!!pendingJob} onChange={event => setUploadTarget(event.target.value as "new" | "selected")}><option value="new">New private draft</option><option value="selected" disabled={!article}>Selected article{article ? `: ${article.title || "Untitled draft"}` : ""}, as a draft revision</option></select></label>
              {/* biome-ignore lint/a11y/noStaticElementInteractions: Drag-and-drop supplements the labeled native file input, which provides the same upload choice to keyboard users. */}
              <div className="dropzone" onDragOver={event => event.preventDefault()} onDrop={dropFile}><div className="empty-icon"><AdminIcon name="import" /></div><h3>Choose a file or drop it here</h3><p>Markdown up to 5 MiB · ZIP up to 50 MiB</p><label htmlFor="markdown-file">Markdown or article ZIP<input ref={fileInput} id="markdown-file" type="file" accept=".md,.zip,text/markdown,application/zip" required={!pendingJob && !file} disabled={!!pendingJob} onChange={event => chooseFile(event.target.files?.[0] ?? null)} /></label></div>
              {file && <div className="selected-file"><AdminIcon name="file" /><div><strong>{file.name}</strong><p>{Math.max(1, Math.round(file.size / 1024))} KB · Ready to import</p></div><button type="button" className="button small" disabled={!!pendingJob} onClick={() => { setFile(null); if (fileInput.current) fileInput.current.value = ""; }}>Remove</button></div>}
              {pendingJob && <p className="help">Import {pendingJob.status}. Resume checks the existing job without uploading another copy.</p>}
              <div className="import-submit"><span className="muted">{uploadTarget === "new" ? "Creates a private, unpublished draft." : "The published version stays unchanged."}</span><button className="button primary" type="submit" disabled={(!file && !pendingJob) || locked}>{busy ? "Importing…" : pendingJob ? "Resume import" : "Upload article"}</button></div>
            </form>
          </fieldset></section><aside className="card"><div className="card-head"><h2>From file to publication</h2></div><div className="card-body"><ol className="steps"><li><strong>Import your article</strong><p>Upload a Markdown file or a ZIP containing one article and its image folders.</p></li><li><strong>Review metadata</strong><p>Check the title, URL, language, writing date, category, and tags.</p></li><li><strong>Preview, then publish</strong><p>Review the imported content. Visibility is a separate setting.</p></li></ol></div><div className="format-note"><p><strong>Including images?</strong></p><p>Keep relative folders in your ZIP. Supported images: PNG, JPEG, GIF, and WebP; up to 10 MiB and 16 million pixels each.</p></div></aside></div>
        </div>
        {article && <div hidden={view !== "editor"}>
          <button className="back" type="button" disabled={locked} onClick={() => navigate("articles")}>← Back to articles</button>
          <div className="page-heading"><div><h1 id="selected-article">{article.title || "Untitled draft"}</h1><p>{article.status === "published" ? "Published article" : "Unpublished draft"} · Version {article.version}</p></div><div className="actions"><button className="button" type="button" disabled={locked} onClick={() => run(async () => { if (await discardChanges()) await selectArticle(article.id); })}>Reload article</button><button className="button" type="button" disabled={locked} onClick={() => { setUploadTarget("selected"); void navigate("import"); }}><AdminIcon name="import" />Import revision</button></div></div>
          <div className="editor-grid"><div className="editor-main"><section className="card" aria-label="Article editor">
            <fieldset className="editor-tabs" aria-label="Article views">{([ ["metadata", "Metadata"], ["preview", "Content preview"], ["history", "Published versions"] ] as const).map(([value, label]) => <button type="button" key={value} disabled={locked || (value === "preview" && !preview)} aria-pressed={editorTab === value} onClick={() => setEditorTab(value)}>{label}</button>)}</fieldset>
            <div hidden={editorTab !== "metadata"}>
              {article.issues.length > 0 && <ul className="admin-issues">{article.issues.map(issue => <li key={`${issue.code}-${issue.path}-${issue.message}`}><strong>{issue.severity === "blocking" ? "Required" : "Note"}:</strong> {issue.message}{issue.path ? ` (${issue.path})` : ""}</li>)}</ul>}
              {article.draft ? <fieldset disabled={locked || !labels} key={article.draft.id}><form ref={metadataForm} className="card-body fields" onSubmit={saveMetadata} onInput={() => { setDirty(true); setPreview(null); }}>
                <label htmlFor="article-title">Article title<input id="article-title" name="title" defaultValue={article.draft.title ?? ""} required /></label>
                <label htmlFor="article-slug">URL name<span className="input-prefix"><span>/articles/</span><input id="article-slug" name="slug" defaultValue={article.slug ?? ""} disabled={!!article.published_at} pattern="[a-z0-9]+(-[a-z0-9]+)*" required /></span></label><p className="help">{article.published_at ? "The URL stays fixed after first publication." : "Use lowercase letters, numbers, and hyphens."}</p>
                <label htmlFor="article-summary">Summary <span className="optional">Optional</span><textarea id="article-summary" name="summary" defaultValue={article.draft.summary ?? ""} /></label>
                <div className="two-col"><label htmlFor="article-language">Language<select id="article-language" name="language" defaultValue={article.draft.language ?? "en"}><option value="en">English</option><option value="zh">中文</option></select></label><label htmlFor="article-written">Writing date<input id="article-written" name="written_at" type="date" defaultValue={article.draft.written_at ?? ""} required /></label></div>
                <label htmlFor="article-updated">Last updated time <span className="optional">Optional · China time (UTC+08:00)</span><input id="article-updated" name="updated_at" type="datetime-local" step="0.001" defaultValue={updatedAtInput(article.draft.updated_at)} aria-describedby="article-updated-help" /></label>
                <p id="article-updated-help" className="help">Set the article’s original last updated time when importing older writing. Leave blank to use the time you publish this revision.</p>
                <label htmlFor="article-category">Category<select id="article-category" name="category_id" defaultValue={article.draft.category?.id ?? ""}><option value="">None</option>{choices("categories").map(label => <option key={label.id} value={label.id}>{label.name}</option>)}</select></label>
                <fieldset><legend>Tags</legend><div className="check-options">{choices("tags").length === 0 ? <p className="help">Create tags in Categories & tags.</p> : choices("tags").map(label => <label key={label.id}><input type="checkbox" name="tag_ids" value={label.id} defaultChecked={article.draft?.tags.some(tag => tag.id === label.id)} />{label.name}</label>)}</div></fieldset>
                <div className="form-foot"><span className={`saved-indicator ${dirty ? "unsaved" : ""}`}>{dirty ? "Unsaved metadata changes" : "No unsaved metadata changes"}</span><button className="button primary" type="submit">Save metadata</button></div>
              </form></fieldset> : <div className="card-body"><h2>Published metadata</h2><p className="help">Import a draft revision to edit this article. The current published version stays unchanged.</p><ArticleMetadata article={article} includeRevisionFields /></div>}
              {article.draft && <section className="admin-metadata-section" aria-labelledby="article-details-title"><h2 id="article-details-title">Article details</h2><p className="help">Dates shown in China time. Publication dates describe the published article.</p><ArticleMetadata article={article} /></section>}
              {!labels && <p className="admin-inline-note">Labels are unavailable. Refresh labels before editing metadata so saved associations are preserved. <button type="button" className="row-action" disabled={locked} onClick={() => run(loadLabels)}>Refresh labels</button></p>}
            </div>
            <div hidden={editorTab !== "preview"}>{preview ? <><div className="preview-label">{preview.id === article.draft?.id ? "Draft preview" : preview.id === article.published?.id ? "Published article" : "Historical version"} · Written {preview.written_at || "Not set"}{preview.first_published_at ? ` · Published ${adminDate(preview.first_published_at)}` : ""}</div><section className="prose admin-preview" id="admin-preview-content" tabIndex={-1} aria-label="Article content preview" lang={preview.language === "zh" ? "zh-CN" : "en"}><h2>{preview.title}</h2><Markdown content={preview.markdown} headings={articleOutline(preview.markdown)} /></section></> : <p className="card-body help">Review the saved draft or read the current published version.</p>}</div>
            <div hidden={editorTab !== "history"}><RevisionHistory key={`${article.id}:${article.published?.id ?? "unpublished"}`} article={article} active={view === "editor" && editorTab === "history"} busy={locked} request={request} run={run} onRead={revision => { setPreview(revision); setEditorTab("preview"); requestAnimationFrame(() => document.getElementById("admin-preview-content")?.focus()); }} onRestored={async restored => {
              setPreview(null); setDirty(false); setEditorTab("metadata");
              try { await selectArticle(restored.id); } catch { setMessage("The version was restored. Reload the article to review its latest draft."); return; }
              try { await loadList(); } catch { setMessage("The version was restored. Review its draft, then reload the article list."); return; }
              setMessage("Version restored as a draft. Review its metadata and preview before publishing.");
              requestAnimationFrame(() => workspace.current?.focus());
            }} /></div>
          </section><AdminNote><strong>Content is written in Typora.</strong> Import Markdown or ZIP to update the body. Saving metadata does not publish it.</AdminNote></div>
          <aside className="editor-side" aria-label="Publication controls"><section className="card"><div className="card-head"><h2>Publication</h2><AdminBadge tone={article.status === "published" ? "green" : "amber"}>{article.status === "published" ? "Published" : "Draft"}</AdminBadge></div><div className="card-body"><p className="help admin-publication-summary">{article.published ? article.draft ? "The published article has an unpublished draft." : "The published version is up to date." : "This article has not been published."}</p><button className="button" type="button" disabled={locked || !article.draft || dirty} onClick={() => run(reviewDraft)}>Review draft preview</button><button className="button primary" type="button" disabled={locked || !canPublish} onClick={() => publication("publish")}>{article.status === "published" ? "Publish changes" : "Publish draft"}</button><p className="help">{dirty ? "Save metadata before reviewing the preview." : !article.draft ? "Import a draft revision to make changes." : !article.publishable ? "Resolve the required fields and import issues before publishing." : canPublish ? `Draft reviewed. Publication visibility: ${article.visibility}.` : "Review the saved draft preview to enable publication."}</p>{article.published && <><button type="button" className="row-action" disabled={locked} onClick={() => { setPreview(article.published); setEditorTab("preview"); }}>Read current version</button><button className="row-action admin-danger-link" type="button" disabled={locked} onClick={() => publication("unpublish")}>Unpublish article</button></>}</div></section>
            <section className="card"><div className="card-head"><h2>Article settings</h2></div><div className="card-body"><div className="setting-row"><div><label htmlFor="article-visibility">Visibility</label><p className="help">{article.visibility === "private" ? "Only the owner can read this article." : "Public readers can read the published version."}</p></div><select id="article-visibility" aria-label="Article visibility" value={article.visibility} disabled={locked} onChange={event => controls(event.target.value as "public" | "private")}><option value="private">Private</option><option value="public">Public</option></select></div><p className="help">Visibility takes effect immediately and does not publish a draft.</p><div className="setting-row"><label className="switch-check"><input type="checkbox" checked={article.comments_enabled} disabled={locked} onChange={toggleComments} />Allow new comments</label></div>{article.published && article.visibility === "public" && <a className="row-action" href={`/articles/${article.slug}`} target="_blank" rel="noreferrer">Read published article ↗</a>}</div></section>

          </aside></div>
        </div>}
        <div hidden={view !== "labels"}><div className="page-heading"><div><h1>Categories & tags</h1><p>Organize all your writing in one collection.</p></div></div><LabelManager labels={labels} busy={locked} request={request} run={run} onChanged={labelsChanged} refresh={loadLabels} /></div>
        <div hidden={view !== "comments"}><div className="page-heading"><div><h1>Comments</h1><p>Review reader submissions before they become public.</p></div></div><CommentModeration article={article} busy={locked} active={view === "comments"} request={request} run={run} onOpenArticle={openArticle} /></div>
      </section>
      <footer className="workspace-foot"><span>iSaber administration</span><span>Dates shown in China time</span></footer>
    </div>
  </div>;
}
