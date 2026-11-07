export type Issue = { severity: "warning" | "blocking"; code: string; path: string | null; message: string };
export type Label = { id: string; name: string; slug: string };
export type AdminLabel = Label & { version: number };
export type Revision = {
  id: string; article_id: string; title: string | null; summary: string | null;
  language: "en" | "zh" | null; written_at: string | null; category: Label | null;
  tags: Label[]; markdown: string; created_at: string; first_published_at: string | null;
  restored_from_revision_id: string | null;
};
export type ArticleSummary = {
  id: string; slug: string | null; title: string | null; status: "draft" | "published";
  visibility: "public" | "private"; has_draft: boolean; version: number;
  published_at: string | null; updated_at: string | null; modified_at: string;
};
export type Article = ArticleSummary & {
  comments_enabled: boolean; draft: Revision | null; published: Revision | null;
  publishable: boolean; issues: Issue[];
};
export type PublicArticle = {
  id: string; slug: string; title: string; summary: string; language: "en" | "zh";
  written_at: string; published_at: string; updated_at: string; category: Label | null;
  tags: Label[]; markdown: string; comments_enabled: boolean;
};
export type Page<T> = { items: T[]; next_cursor: string | null };
export type PublicArticleSummary = Omit<PublicArticle, "markdown" | "comments_enabled">;
export type ImportJob = {
  id: string; status: "queued" | "processing" | "succeeded" | "failed";
  article_id: string | null; revision_id: string | null; publishable: boolean;
  issues: Issue[]; created_at: string; finished_at: string | null;
};
