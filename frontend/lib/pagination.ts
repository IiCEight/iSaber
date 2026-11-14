export type PageToken = number | "before" | "after";

export function pageTokens(current: number, total: number): PageToken[] {
  if (total <= 7) return Array.from({ length: total }, (_, index) => index + 1);
  let start = Math.max(2, current - 1), end = Math.min(total - 1, current + 1);
  if (current <= 4) { start = 2; end = 5; }
  if (current >= total - 3) { start = total - 4; end = total - 1; }
  return [1, ...(start > 2 ? ["before" as const] : []), ...Array.from({ length: end - start + 1 }, (_, index) => start + index), ...(end < total - 1 ? ["after" as const] : []), total];
}

export function pageHref(basePath: string, query: string, page: number): string {
  const params = new URLSearchParams(query);
  params.delete("cursor"); params.delete("page");
  if (page > 1) params.set("page", String(page));
  return `${basePath}${params.size ? `?${params}` : ""}${basePath === "/archive" ? "#archive-writing" : ""}`;
}
