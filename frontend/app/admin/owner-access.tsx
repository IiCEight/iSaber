"use client";

import { useState, type FormEvent } from "react";

export type OwnerSession = {
  owner: { id: string; username: string };
  csrf_token: string;
  expires_at: string;
};

export default function OwnerAccess({ initialSession, unavailable }: {
  initialSession: OwnerSession | null;
  unavailable: boolean;
}) {
  const [session, setSession] = useState(initialSession);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState(unavailable ? "Sign-in is temporarily unavailable. Please try again." : "");

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

  return (
    <section aria-labelledby="owner-title" className="owner-access">
      <h1 id="owner-title">{session ? "Your dashboard" : "Owner sign-in"}</h1>
      {message && <p role="alert">{message}</p>}
      {session ? (
        <>
          <p>Signed in as <strong>{session.owner.username}</strong>.</p>
          <p>Your publishing tools are on their way.</p>
          <button type="button" disabled={busy} onClick={signOut}>{busy ? "Signing out…" : "Sign out"}</button>
        </>
      ) : (
        <form method="post" action="/api/v1/auth/login" onSubmit={signIn} aria-busy={busy}>
          <label htmlFor="owner-username">Username</label>
          <input id="owner-username" name="username" autoComplete="username" required disabled={busy} />
          <label htmlFor="owner-password">Password</label>
          <input id="owner-password" name="password" type="password" autoComplete="current-password" required disabled={busy} />
          <button type="submit" disabled={busy}>{busy ? "Signing in…" : "Sign in"}</button>
        </form>
      )}
    </section>
  );
}
