import type { Article } from "../../lib/articles";
import { adminDateTime } from "./admin-ui";

function timestamp(value: string | null, fallback: string) {
  return value ? <time dateTime={value}>{adminDateTime(value)}</time> : fallback;
}

export default function ArticleMetadata({ article, includeRevisionFields = false }: { article: Article; includeRevisionFields?: boolean }) {
  const revision = article.draft ?? article.published;
  return <dl className="admin-metadata-grid">
    {includeRevisionFields && <>
      <div><dt>Title</dt><dd>{revision?.title || "Untitled draft"}</dd></div>
      <div><dt>Article URL</dt><dd>{article.slug ? `/articles/${article.slug}` : "Not set"}</dd></div>
      <div className="admin-metadata-wide"><dt>Summary</dt><dd>{revision?.summary || "No summary"}</dd></div>
      <div><dt>Language</dt><dd>{revision?.language === "zh" ? "中文" : revision?.language === "en" ? "English" : "Not set"}</dd></div>
      <div><dt>Category</dt><dd>{revision?.category?.name || "None"}</dd></div>
      <div className="admin-metadata-wide"><dt>Tags</dt><dd>{revision?.tags.map(tag => tag.name).join(", ") || "None"}</dd></div>
    </>}
    <div><dt>Written</dt><dd>{revision?.written_at ? <time dateTime={revision.written_at}>{revision.written_at}</time> : "Not set"}</dd></div>
    <div><dt>First published</dt><dd>{timestamp(article.published_at, "Not published")}</dd></div>
    <div><dt>Last updated</dt><dd>{timestamp(article.updated_at, "Not published")}</dd></div>
    <div><dt>Modified in workspace</dt><dd>{timestamp(article.modified_at, "Not set")}</dd></div>
    <div><dt>Status</dt><dd>{article.status === "published" ? "Published" : "Draft"}{article.draft && article.published ? " · Unpublished changes" : ""}</dd></div>
    <div><dt>Visibility</dt><dd>{article.visibility === "public" ? "Public" : "Private"}</dd></div>
    <div><dt>New comments</dt><dd>{article.comments_enabled ? "Allowed" : "Disabled"}</dd></div>
    <div><dt>Version</dt><dd>{article.version}</dd></div>
  </dl>;
}
