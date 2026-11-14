import Link from "next/link";
import { pageHref } from "../lib/pagination";
import { articleSortFields, defaultArticleSort, validArticleSort } from "../lib/article-sort";

export default function ArchiveSort({ query, searching }: { query: string; searching: boolean }) {
  const applied = new URLSearchParams(query);
  const order = validArticleSort(applied.get("sort"), searching) ? applied.get("sort") ?? defaultArticleSort : defaultArticleSort;
  const ascending = articleSortFields.some(field => field.oldest === order);
  const selected = articleSortFields.find(field => field.newest === order || field.oldest === order) ?? articleSortFields[0];
  const href = (value: string) => {
    const next = new URLSearchParams(query);
    next.set("sort", value);
    return pageHref("/archive", next.toString(), 1);
  };
  return <nav className="archive-sort" aria-label="Sort articles">
    <span className="archive-sort-caption">Sort by</span>
    <div className="archive-sort-fields">
      {articleSortFields.map(field => <Link key={field.newest} className="archive-sort-field" href={href(ascending ? field.oldest : field.newest)} prefetch={false} aria-current={order === field.newest || order === field.oldest ? "true" : undefined} aria-label={`Sort by ${field.dateLabel.toLowerCase()}`}>{field.label}</Link>)}
      {searching && <Link className="archive-sort-field" href={href("relevance")} prefetch={false} aria-current={order === "relevance" ? "true" : undefined}>Best match</Link>}
    </div>
    {order !== "relevance" && <Link className="archive-sort-direction" href={href(ascending ? selected.newest : selected.oldest)} prefetch={false} aria-label={`Switch to ${ascending ? "newest" : "oldest"} first`}>
      <span>{ascending ? "Oldest first" : "Newest first"}</span><span className="archive-sort-arrow" aria-hidden="true">{ascending ? "↑" : "↓"}</span>
    </Link>}
  </nav>;
}
