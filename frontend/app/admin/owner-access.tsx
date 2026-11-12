"use client";

import { useCallback, useEffect, useState, type FormEvent } from "react";
import PublishingDashboard from "./publishing-dashboard";

export type OwnerSession = {
  owner: { id: string; username: string };
  csrf_token: string;
  expires_at: string;
};

export default function OwnerAccess({ initialSession, unavailable }: {
  initialSession: OwnerSession | null;
  unavailable: boolean;
}) {
  const [ready, setReady] = useState(false);
  useEffect(() => { setReady(true); }, []);
  const [session, setSession] = useState(initialSession);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState(unavailable ? "Sign-in is temporarily unavailable. Please try again." : "");
  const expired = useCallback(() => { setSession(null); setMessage("Your session expired. Sign in again; saved drafts are kept."); }, []);

  async function signIn(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const fields = new FormData(form);
    setBusy(true);
    setMessage("");
    try {
      const response = await fetch("/api/v1/auth/login", {
        method: "POST",
        credentials: "same-origin",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username: fields.get("username"), password: fields.get("password") }),
        signal: AbortSignal.timeout(10000),
      });
      if (response.ok) {
        const next: OwnerSession = await response.json();
        form.reset();
        setSession(next);
      } else if (response.status === 401) {
        setMessage("The username or password is incorrect.");
      } else if (response.status === 429) {
        setMessage("Too many sign-in attempts. Please wait a minute before trying again.");
      } else if (response.status === 400 || response.status === 422) {
        setMessage("Enter a valid username and password.");
      } else {
        setMessage("Sign-in is temporarily unavailable. Please try again.");
      }
    } catch {
      setMessage("Sign-in could not be confirmed. Please try again.");
    } finally {
      const password = form.elements.namedItem("password");
      if (password instanceof HTMLInputElement) password.value = "";
      setBusy(false);
    }
  }

  async function signOut() {
    if (!session) return;
    setBusy(true);
    setMessage("");
    try {
      const response = await fetch("/api/v1/auth/logout", {
        method: "POST",
        credentials: "same-origin",
        headers: { "X-CSRF-Token": session.csrf_token },
        signal: AbortSignal.timeout(10000),
      });
      if (response.ok || response.status === 401) setSession(null);
      else setMessage("Sign-out could not be confirmed. Please try again.");
    } catch {
      setMessage("Sign-out could not be confirmed. Please try again.");
    } finally {
      setBusy(false);
    }
  }

  if (session) return <PublishingDashboard csrf={session.csrf_token} onExpired={expired} username={session.owner.username} signingOut={busy} onSignOut={signOut} sessionMessage={message} />;
  return <section className="admin-signin" aria-labelledby="owner-title">
    <div className="admin-login-top"><a className="brand" href="/">iSaber</a><a href="/">← Back to blog</a></div>
    <div className="signin-wrap"><div className="card"><div className="card-body">
      <p className="eyebrow">YOUR CONTENT WORKSPACE</p><h1 id="owner-title">Owner sign-in</h1>
      <p className="muted">Manage your writing and reader comments.</p>
      {message && <p className="admin-alert" role="alert">{message}</p>}
      <noscript>Enable JavaScript to sign in and manage your articles.</noscript>
      <form method="post" action="/api/v1/auth/login" onSubmit={signIn} aria-busy={busy}>
        <label htmlFor="owner-username">Username</label>
        <input id="owner-username" name="username" autoComplete="username" required disabled={busy || !ready} />
        <label htmlFor="owner-password">Password</label>
        <input id="owner-password" name="password" type="password" autoComplete="current-password" required disabled={busy || !ready} />
        <button className="button primary" type="submit" disabled={busy || !ready}>{busy ? "Signing in…" : "Sign in"}</button>
      </form>
      <p className="help">One owner account. No reader registration.</p>
    </div></div></div>
  </section>;
}
