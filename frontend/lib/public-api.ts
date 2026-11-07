import { cache } from "react";
import type { Label, Page, PublicArticle } from "./articles";

export function backendOrigin() {
  return new URL(process.env.BACKEND_ORIGIN ?? "http://127.0.0.1:8080").origin;
}

// React memoizes within the render request; no shared cross-request cache is used.
export const getPublicArticle = cache(async (slug: string): Promise<PublicArticle | null> => {
  const response = await fetch(`${backendOrigin()}/api/v1/articles/${encodeURIComponent(slug)}`, { cache: "no-store", signal: AbortSignal.timeout(5000) });
  if (response.status === 404) return null;
  if (!response.ok) throw new Error("Article temporarily unavailable");
  return response.json();
});

export async function getPublicLabels(kind: "categories" | "tags") {
  const found = new Map<string, Label>();
  let cursor: string | null = null;
  do {
    const query = new URLSearchParams({ limit: "100" }); if (cursor) query.set("cursor", cursor);
    const response = await fetch(`${backendOrigin()}/api/v1/${kind}?${query}`, { cache: "no-store", signal: AbortSignal.timeout(5000) });
    if (!response.ok) throw new Error("Labels temporarily unavailable");
    const page: Page<Label> = await response.json();
    for (const label of page.items) found.set(label.id, label);
    cursor = page.next_cursor;
  } while (cursor);
  return [...found.values()];
}
