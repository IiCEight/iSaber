import Link from "next/link";
import { backendOrigin, getPublicLabels } from "../lib/public-api";
import type { Label, Page, PublicArticleSummary } from "../lib/articles";

type Params = { q?: string | string[]; cursor?: string; category?: string; tag?: string; language?: string };
export default async function Home({ searchParams }: { searchParams: Promise<Params> }) {
  const params = await searchParams;
  const query = new URLSearchParams();
  for (const key of ["category", "tag", "language", "cursor"] as const) if (typeof params[key] === "string" && params[key]) query.set(key, params[key]);
  const searching = params.q !== undefined;
  const malformedQuery = searching && typeof params.q !== "string";
  const rawQuery = typeof params.q === "string" ? params.q : "";
  const text = rawQuery.trim();
  if (searching) query.set("q", text);
  const emptySearch = searching && !malformedQuery && text === "";
  const tooLong = searching && Array.from(rawQuery).length > 200;
  const filters = ["category", "tag", "language"].some((key) => query.has(key));
  const [articlesResult, categoriesResult, tagsResult] = await Promise.allSettled([
    malformedQuery || emptySearch || tooLong ? Promise.resolve({ status: 400, page: null }) : fetch(`${backendOrigin()}/api/v1/articles${searching ? "/search" : ""}?${query}`, { cache: "no-store", signal: AbortSignal.timeout(5000) }).then(async (response) => ({ status: response.status, page: response.ok ? await response.json() as Page<PublicArticleSummary> : null })),
    getPublicLabels("categories"), getPublicLabels("tags"),
  ]);
  const page = articlesResult.status === "fulfilled" ? articlesResult.value.page : null;
  const invalid = articlesResult.status === "fulfilled" && articlesResult.value.status === 400;
  const categories: Label[] = categoriesResult.status === "fulfilled" ? categoriesResult.value : [];
  const tags: Label[] = tagsResult.status === "fulfilled" ? tagsResult.value : [];
  const nextQuery = new URLSearchParams(query);
  if (page?.next_cursor) nextQuery.set("cursor", page.next_cursor);
  return (
    <>
      <p>Journals · Technology · Selected work</p>
      <h1>A home for words and ideas.</h1>
      <section aria-labelledby="latest-articles">
        <h2 id="latest-articles">{searching ? "Search results" : "Articles"}</h2>
        <search aria-label="Search public articles"><form action="/" method="get" key={text}>
          <p>Search titles, summaries, tags, and article text. Separate terms with spaces; all terms must match.</p>
          <label htmlFor="article-search">Search articles</label><input id="article-search" type="search" name="q" defaultValue={text} maxLength={200} />
          {["category", "tag", "language"].map((key) => query.has(key) ? <input key={key} type="hidden" name={key} value={query.get(key) ?? ""} /> : null)}
          <button type="submit">Search</button>{searching && <Link href="/">Browse all articles</Link>}
        </form></search>
        <form action="/" method="get" className="article-filters" key={`${query.get("category")}:${query.get("tag")}:${query.get("language")}`}>
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
          <button type="submit">Filter articles</button><Link href={searching && text ? `/?q=${encodeURIComponent(text)}` : "/"}>Clear filters</Link>
        </form>
        {(categoriesResult.status === "rejected" || tagsResult.status === "rejected") && <p>Some filter options are temporarily unavailable.</p>}
        {malformedQuery ? <p>Use a single search query.</p> : emptySearch ? <p>Enter a word or phrase to search public articles.</p> : tooLong ? <p>Use a search query of at most 200 characters.</p> : !page ? <p>{invalid ? "Check the query and filters, then try again." : searching ? "Search is temporarily unavailable. Please try again shortly." : "Articles are temporarily unavailable. Please try again shortly."}</p> : page.items.length === 0 ? <p>{searching ? "No public articles match this search." : filters ? "No articles match these filters." : "The first chapter is on its way."}</p> : <ul>{page.items.map((article) => <li key={article.id}><h3><Link href={`/articles/${article.slug}`}>{article.title}</Link></h3><p>{article.summary}</p><time dateTime={article.published_at}>{new Date(article.published_at).toLocaleDateString("en", { timeZone: "Asia/Shanghai" })}</time></li>)}</ul>}
        {page?.next_cursor && <Link href={`/?${nextQuery}`}>{searching ? "More search results" : "More articles"}</Link>}
      </section>
    </>
  );
}
