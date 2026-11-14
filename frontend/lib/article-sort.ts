export const defaultArticleSort = "written";

export const articleSortFields = [
  { label: "Written", dateLabel: "Written date", newest: "written", oldest: "written_oldest" },
  { label: "Last updated", dateLabel: "Last updated", newest: "updated", oldest: "updated_oldest" },
  { label: "Published", dateLabel: "Published date", newest: "newest", oldest: "oldest" },
] as const;

export function validArticleSort(value: unknown, searching: boolean): boolean {
  return typeof value === "string" && (articleSortFields.some(field => field.newest === value || field.oldest === value) || (searching && value === "relevance"));
}
