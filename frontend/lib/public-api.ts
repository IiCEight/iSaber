import { cache } from "react";
import type { PublicArticle } from "./articles";

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
