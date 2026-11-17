import "server-only";
import { cache } from "react";
import { headers } from "next/headers";
import { backendOrigin, getPublicArticle, getPublicLabels } from "./public-api";
import type { Label, Page, ReadingArticle, ReadingArticleSummary } from "./articles";

export const getReadingAccess = cache(async () => {
  // Preserve duplicate session cookies so the Go authentication boundary can
  // reject ambiguity. Only the two supported session namespaces are forwarded.
  const raw = (await headers()).get("cookie") ?? "";
  const cookie = raw.split(";").map(item => item.trim()).filter(item => item.startsWith("__Host-isaber_session=") || item.startsWith("isaber_dev_session=")).join("; ");
  if (!cookie) return null;
  try {
    const response = await fetch(`${backendOrigin()}/api/v1/auth/session`, { headers: { Cookie: cookie }, cache: "no-store", signal: AbortSignal.timeout(5000) });
    if (!response.ok) return null;
    const session = await response.json() as { expires_at: string };
    if (typeof session.expires_at !== "string" || !Number.isFinite(Date.parse(session.expires_at))) return null;
    return { cookie, expiresAt: session.expires_at };
  } catch { return null; }
});

// Cookies are sent only to protected endpoints. A revoked session falls back
// to the public endpoint, which cannot return private data.
export async function readingFetch(path: string) {
  const access = await getReadingAccess();
  if (access) {
    const response = await fetch(`${backendOrigin()}/api/v1/admin/reading/${path}`, { headers: { Cookie: access.cookie }, cache: "no-store", signal: AbortSignal.timeout(5000) });
    if (response.status !== 401) return response;
  }
  return fetch(`${backendOrigin()}/api/v1/${path}`, { cache: "no-store", signal: AbortSignal.timeout(5000) });
}

export const getReadingArticle = cache(async (slug: string): Promise<ReadingArticle | null> => {
  if (!await getReadingAccess()) return getPublicArticle(slug);
  const response = await readingFetch(`articles/${encodeURIComponent(slug)}`);
  if (response.status === 404) return null;
  if (!response.ok) throw new Error("Article temporarily unavailable");
  return response.json();
});

export async function getReadingLabels(kind: "categories" | "tags") {
  if (!await getReadingAccess()) return getPublicLabels(kind);
  const found = new Map<string, Label>();
  let cursor: string | null = null;
  do {
    const query = new URLSearchParams({ limit: "100" }); if (cursor) query.set("cursor", cursor);
    const response = await readingFetch(`${kind}?${query}`);
    if (!response.ok) throw new Error("Labels temporarily unavailable");
    const page: Page<Label> = await response.json();
    for (const label of page.items) found.set(label.id, label);
    cursor = page.next_cursor;
  } while (cursor);
  return [...found.values()];
}

export async function readingPage(query: string, searching: boolean, category?: string) {
  const response = await readingFetch(`${category ? `categories/${encodeURIComponent(category)}/articles` : `articles${searching ? "/search" : ""}`}?${query}`);
  return { status: response.status, page: response.ok ? await response.json() as Page<ReadingArticleSummary> : null };
}
