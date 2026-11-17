import Link from "next/link";
import PinBadge from "./pin-badge";
import { redirect } from "next/navigation";
import { pageHref } from "../lib/pagination";
import { validArticleSort } from "../lib/article-sort";
import ArchiveView from "./archive-view";
import { getReadingAccess, getReadingLabels, readingPage } from "../lib/reading-api";
import type { Category, Label, NumberedPage, ReadingArticleSummary } from "../lib/articles";

export type BrowseParams = { q?: string | string[]; cursor?: string; page?: string | string[]; sort?: string | string[]; category?: string; tag?: string; language?: string };
export default async function ArticleBrowser({ params, basePath, category }: { params: BrowseParams; basePath: string; category?: Category }) {
  const access = await getReadingAccess();
  const query = new URLSearchParams();
  for (const key of ["category", "tag", "language", "cursor"] as const) if ((!category || key !== "category") && typeof params[key] === "string" && params[key]) query.set(key, params[key]);
  const rawQuery = typeof params.q === "string" ? params.q : "";
  const text = rawQuery.trim();
  const searching = params.q !== undefined && (basePath === "/search" || typeof params.q !== "string" || text !== "");
  const malformedQuery = searching && typeof params.q !== "string";
  if (searching) query.set("q", text);
  const emptySearch = searching && !malformedQuery && text === "";
  const tooLong = searching && Array.from(rawQuery).length > 200;
  const filters = ["category", "tag", "language"].some((key) => query.has(key));
  const numbered = basePath === "/archive" || !!category;
  const validSort = params.sort === undefined || validArticleSort(params.sort, searching);
  if (numbered && typeof params.sort === "string") query.set("sort", params.sort);
  if (numbered && params.sort === "relevance" && !searching) { query.delete("sort"); redirect(pageHref(basePath, query.toString(), 1)); }
  const validPage = params.page === undefined || (typeof params.page === "string" && /^[1-9][0-9]*$/.test(params.page) && Number(params.page) <= 2147483647);
  const requestedPage = validPage ? Number(params.page ?? "1") : 1;
  if (numbered && params.cursor) redirect(pageHref(basePath, query.toString(), 1));
  if (numbered && requestedPage > 1) query.set("page", String(requestedPage));
  const apiQuery = new URLSearchParams(query);
  if (numbered) { apiQuery.set("page", String(requestedPage)); apiQuery.set("limit", "10"); }
  const [articlesResult, categoriesResult, tagsResult] = await Promise.allSettled([
    malformedQuery || emptySearch || tooLong || (numbered && (!validPage || !validSort)) ? Promise.resolve({ status: 400, page: null }) : readingPage(apiQuery.toString(), searching, category?.slug),
    getReadingLabels("categories"), getReadingLabels("tags"),
  ]);
  const page = articlesResult.status === "fulfilled" ? articlesResult.value.page : null;
  const invalid = articlesResult.status === "fulfilled" && articlesResult.value.status === 400;
  const categories: Label[] = categoriesResult.status === "fulfilled" ? categoriesResult.value : [];
  const tags: Label[] = tagsResult.status === "fulfilled" ? tagsResult.value : [];
  const nextQuery = new URLSearchParams(query);
  if (page?.next_cursor) nextQuery.set("cursor", page.next_cursor);
  if (numbered) {
    const collection = page && "page" in page && "total" in page && "page_size" in page && "total_pages" in page ? page as NumberedPage<ReadingArticleSummary> : null;
    if (collection && collection.page !== requestedPage) redirect(pageHref(basePath, query.toString(), collection.page));
    const error = !validPage ? "Use a positive whole page number." : !validSort ? "Use a valid article order." : malformedQuery ? "Use a single search query." : tooLong ? "Use a search query of at most 200 characters." : !collection ? invalid ? "Check the query and filters, then try again." : searching ? "Search is temporarily unavailable. Please try again shortly." : "Articles are temporarily unavailable. Please try again shortly." : null;
    return <ArchiveView category={category} basePath={basePath} page={collection} query={query.toString()} categories={categories} tags={tags} labelsUnavailable={categoriesResult.status === "rejected" || tagsResult.status === "rejected"} error={error} searching={searching} filtered={filters} owner={!!access} />;
  }
  return (
    <div className="article-browser">
      <p className="page-eyebrow">iSaber · {access ? "Your published articles" : "Public articles"}</p>
      <h1>{searching ? "Search articles" : "Archive"}</h1>
      <section aria-labelledby="latest-articles">
        <h2 id="latest-articles">{searching ? "Search results" : "Articles"}</h2>
        <search aria-label={access ? "Search your articles" : "Search public articles"}><form action="/search" method="get" key={text} className="archive-search-form">
          <p>Search titles, summaries, tags, and article text. Separate terms with spaces; all terms must match.</p>
          <label htmlFor="article-search">Search articles</label><input id="article-search" type="search" name="q" defaultValue={text} maxLength={200} />
          {["category", "tag", "language"].map((key) => query.has(key) ? <input key={key} type="hidden" name={key} value={query.get(key) ?? ""} /> : null)}
          <button type="submit">Search</button>{searching && <Link href="/archive">Browse all articles</Link>}
        </form></search>
        <form action={basePath} method="get" className="article-filters" key={`${query.get("category")}:${query.get("tag")}:${query.get("language")}`}>
          {searching && <input type="hidden" name="q" value={text} />}
          <label>Category<select name="category" defaultValue={query.get("category") ?? ""}>
            <option value="">All categories</option>{query.has("category") && !categories.some((label) => label.slug === query.get("category")) && <option value={query.get("category") ?? ""}>Selected category</option>}
            {categories.map((label) => <option key={label.id} value={label.slug}>{label.name}</option>)}
          </select></label>
          <label>Tag<select name="tag" defaultValue={query.get("tag") ?? ""}>
            <option value="">All tags</option>{query.has("tag") && !tags.some((label) => label.slug === query.get("tag")) && <option value={query.get("tag") ?? ""}>Selected tag</option>}
            {tags.map((label) => <option key={label.id} value={label.slug}>{label.name}</option>)}
          </select></label>
          <label>Language<select name="language" defaultValue={query.get("language") ?? ""}><option value="">English and Chinese</option><option value="en">English</option><option value="zh">中文</option></select></label>
          <button type="submit">Filter articles</button><Link href={searching && text ? `/search?q=${encodeURIComponent(text)}` : "/archive"}>Clear filters</Link>
        </form>
        {(categoriesResult.status === "rejected" || tagsResult.status === "rejected") && <p>Some filter options are temporarily unavailable.</p>}
        {malformedQuery ? <p>Use a single search query.</p> : emptySearch ? <p>{access ? "Enter a word or phrase to search articles." : "Enter a word or phrase to search public articles."}</p> : tooLong ? <p>Use a search query of at most 200 characters.</p> : !page ? <p>{invalid ? "Check the query and filters, then try again." : searching ? "Search is temporarily unavailable. Please try again shortly." : "Articles are temporarily unavailable. Please try again shortly."}</p> : page.items.length === 0 ? <p>{searching ? access ? "No articles match this search." : "No public articles match this search." : filters ? "No articles match these filters." : "The first chapter is on its way."}</p> : <ul>{page.items.map((article) => <li key={article.id}><h3 lang={article.language === "zh" ? "zh-CN" : "en"}><Link href={`/articles/${article.slug}`}>{article.title}</Link></h3><p>{article.summary}</p>{article.pinned && <PinBadge />}{article.visibility === "private" && <span className="private-writing-label">Private</span>}<time dateTime={article.published_at}>{new Date(article.published_at).toLocaleDateString("en", { timeZone: "Asia/Shanghai" })}</time></li>)}</ul>}
        {page?.next_cursor && <Link href={`${basePath}?${nextQuery}`}>{searching ? "More search results" : "More articles"}</Link>}
      </section>
    </div>
  );
}
