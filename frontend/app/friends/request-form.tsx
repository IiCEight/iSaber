"use client";
import { useEffect, useRef, useState, type FormEvent } from "react";
import Image from "next/image";
import FriendIcon from "../../components/friend-icon";
import { friendImageURL, type FriendDetails } from "../../lib/friends";

type Draft = FriendDetails & { email: string };
const empty: Draft = { site_name: "", site_url: "", introduction: "", icon_url: "", email: "" };
export function FriendIdentity() {
  const [message, setMessage] = useState("");
  const [ready, setReady] = useState(false);
  useEffect(() => setReady(true), []);
  return <div className="friends-site-card"><div className="friends-site-identity"><Image unoptimized src="/isaber-profile.jpg" alt="" width={56} height={56} /><div><h3>iSaber</h3><a href="https://isaber.top" target="_blank" rel="noopener noreferrer">https://isaber.top <span aria-hidden="true">↗</span><span className="friends-sr-only"> (opens in a new tab)</span></a></div></div><p>Personal journals, technical notes, and things I make.</p><button type="button" disabled={!ready} onClick={async () => { try { await navigator.clipboard.writeText("https://isaber.top"); setMessage("Site link copied."); } catch { setMessage("Copy unavailable. Select and copy the URL above."); } }}>Copy my site link <span aria-hidden="true">↗</span></button><p role="status" id="copy-site-status">{message}</p></div>;
}
export default function FriendRequestForm() {
  const [draft, setDraft] = useState<Draft>(empty);
  const [image, setImage] = useState("");
  const [ready, setReady] = useState(false), [busy, setBusy] = useState(false), [uncertain, setUncertain] = useState(false);
  const [message, setMessage] = useState(""), [wait, setWait] = useState(0);
  const dialog = useRef<HTMLDialogElement>(null), preview = useRef<HTMLButtonElement>(null);
  const attempt = useRef<{ key: string; payload: Draft } | null>(null);
  useEffect(() => setReady(true), []);
  useEffect(() => { if (wait <= 0) return; const timer = setTimeout(() => setWait(wait - 1), 1000); return () => clearTimeout(timer); }, [wait]);
  function update(key: keyof Draft, value: string) { setDraft(old => ({ ...old, [key]: value })); if (key === "icon_url") setImage(""); }
  function review(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!draft.site_name.trim() || !draft.introduction.trim()) { setMessage("Enter a website name and a short introduction."); return; }
    if (!friendImageURL(draft.site_url) || (draft.icon_url.trim() && !friendImageURL(draft.icon_url))) { setMessage("Use HTTPS URLs without a username or password for the site and optional icon."); return; }
    setImage(friendImageURL(draft.icon_url));
    dialog.current?.showModal();
  }
  async function send() {
    if (busy || wait > 0) return;
    if (!attempt.current) attempt.current = { key: crypto.randomUUID(), payload: Object.fromEntries(Object.entries(draft).map(([key, value]) => [key, value.trim()])) as Draft };
    const current = attempt.current;
    setBusy(true); setMessage("");
    try {
      const response = await fetch("/api/v1/friend-requests", { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": current.key }, body: JSON.stringify(current.payload), cache: "no-store", credentials: "same-origin", signal: AbortSignal.timeout(10000) });
      if (response.status === 202) {
        attempt.current = null; setUncertain(false); setDraft(empty); setImage(""); setMessage("Thank you. Your request is waiting for review. It will appear only after approval and publication."); dialog.current?.close(); return;
      }
      const problem = await response.json().catch(() => ({ code: "request_failed" }));
      const unknown = response.status >= 500 || problem.code === "request_in_progress" || problem.code === "idempotency_conflict";
      setUncertain(unknown);
      if (!unknown) attempt.current = null;
      if (response.status === 429) setWait(Math.min(600, Math.max(1, Number(response.headers.get("Retry-After")) || 60)));
      const messages: Record<string, string> = { invalid_friend: "Check the name (80 characters) and introduction (300 characters). Your text is kept.", invalid_friend_url: "Use direct HTTPS URLs without credentials. Your text is kept.", invalid_reply_email: "Check your reply email address, or leave it blank.", invalid_origin: "Open the blog at its configured address to send a request.", friend_request_rate_limited: "Too many recent requests. Please wait before retrying." };
      setMessage(messages[problem.code] ?? "Submission could not be confirmed. Your text is kept; retry the same request.");
    } catch { setUncertain(true); setMessage("Submission could not be confirmed. Your text is kept; retry the same request."); }
    finally { setBusy(false); }
  }
  return <div className="friends-request-form-surface"><h3>Say hello.</h3><p className="friends-request-preview-note" id="request-note">Requests are reviewed before a link appears. Your reply email stays private.</p>
    {message && <p role="status">{message}</p>}
    <form id="friend-request-form" method="post" action="/api/v1/friend-requests" aria-describedby="request-note" onSubmit={review}>
      <fieldset disabled={!ready || busy || uncertain} className="friends-request-fields-container"><legend className="friends-sr-only">Your friend-link request</legend>
      <div className="friends-request-fields"><label>Website name *<input name="site_name" value={draft.site_name} onChange={e => update("site_name", e.target.value)} maxLength={80} required placeholder="Your little corner" /></label><label>Website URL *<input name="site_url" type="url" inputMode="url" autoComplete="url" value={draft.site_url} onChange={e => update("site_url", e.target.value)} maxLength={2048} required placeholder="https://your-site.example" /><span className="friends-field-help">Use the HTTPS address of your site.</span></label></div>
      <label>A short introduction *<textarea name="introduction" value={draft.introduction} onChange={e => update("introduction", e.target.value)} rows={3} maxLength={300} required placeholder="A few words about you and your site…" /><span className="friends-field-help">Up to 300 characters. English or 中文.</span></label>
      <div className="friends-icon-field"><label>Profile icon URL <span className="friends-field-optional">(optional)</span><input name="icon_url" type="url" inputMode="url" value={draft.icon_url} onChange={e => update("icon_url", e.target.value)} onBlur={() => setImage(friendImageURL(draft.icon_url))} maxLength={2048} placeholder="https://your-site.example/icon.png" /><span className="friends-field-help">A direct HTTPS image URL. A square image around 256 × 256 works well. Leave blank to use initials.</span></label><div className="friends-icon-preview"><FriendIcon name={draft.site_name} url={image} /><span className="friends-icon-status">{image ? "Image preview; initials are used if it cannot load." : "Initials will be used."}</span>{draft.icon_url && <button type="button" onClick={() => { update("icon_url", ""); setImage(""); }}>Remove icon</button>}</div></div>
      <label>Reply email <span className="friends-field-optional">(optional)</span><input name="email" type="email" autoComplete="email" value={draft.email} onChange={e => update("email", e.target.value)} maxLength={254} placeholder="you@example.com" /><span className="friends-field-help">For a reply; not part of your public listing.</span></label>
      </fieldset>
      <div className="friends-request-actions"><p>Requests will be reviewed before a link appears.</p><button type="submit" ref={preview} disabled={!ready || busy || wait > 0}>{wait > 0 ? `Retry in ${wait}s` : uncertain ? "Retry same request ↗" : "Preview request ↗"}</button></div>
      <noscript><p>Enable JavaScript to preview and send a friend-link request.</p></noscript>
    </form>
    <dialog className="friends-sample-dialog" ref={dialog} aria-labelledby="request-preview-title" onClose={() => preview.current?.focus()} onCancel={event => { if (busy) event.preventDefault(); }}>
      <p className="friends-eyebrow">Review your request</p><h2 id="request-preview-title">Your little corner.</h2><p>Sending adds a private request to the owner’s review queue.</p><dl className="friends-request-summary"><dt>Profile icon</dt><dd><FriendIcon name={draft.site_name} url={image} /></dd><dt>Website</dt><dd>{draft.site_name}</dd><dt>URL</dt><dd>{draft.site_url}</dd><dt>Introduction</dt><dd>{draft.introduction}</dd>{draft.icon_url && <><dt>Icon URL</dt><dd>{draft.icon_url}</dd></>}{draft.email && <><dt>Reply email (private)</dt><dd>{draft.email}</dd></>}</dl>
      {message && <p role="status">{message}</p>}
      <button type="button" disabled={busy} onClick={() => dialog.current?.close()}>Keep editing</button>{" "}<button type="button" disabled={busy || wait > 0} onClick={() => void send()}>{busy ? "Sending…" : wait > 0 ? `Retry in ${wait}s` : uncertain ? "Retry same request" : "Send for review"}</button>
    </dialog>
  </div>;
}
