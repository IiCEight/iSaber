import Link from "next/link";
import { backendOrigin } from "../lib/public-api";
import type { Page, PublicArticleSummary } from "../lib/articles";

export default async function Home({ searchParams }: { searchParams: Promise<{ cursor?: string }> }) {
  let page: Page<PublicArticleSummary> | null = null;
  try {
    const query = new URLSearchParams();
    const cursor = (await searchParams).cursor;
    if (cursor) query.set("cursor", cursor);
    const response = await fetch(`${backendOrigin()}/api/v1/articles?${query}`, { cache: "no-store", signal: AbortSignal.timeout(5000) });
    if (response.ok) page = await response.json();
  } catch { /* The public shell stays available if the API is temporarily offline. */ }
  return (
    <>
      <p>Journals · Technology · Selected work</p>
      <h1>A home for words and ideas.</h1>
      <section aria-labelledby="latest-articles">
        <h2 id="latest-articles">Articles</h2>
        {!page ? <p>Articles are temporarily unavailable. Please try again shortly.</p> : page.items.length === 0 ? <p>The first chapter is on its way.</p> : <ul>{page.items.map((article) => <li key={article.id}><h3><Link href={`/articles/${article.slug}`}>{article.title}</Link></h3><p>{article.summary}</p><time dateTime={article.published_at}>{new Date(article.published_at).toLocaleDateString("en", { timeZone: "Asia/Shanghai" })}</time></li>)}</ul>}
        {page?.next_cursor && <Link href={`/?cursor=${encodeURIComponent(page.next_cursor)}`}>More articles</Link>}
      </section>
    </>
  );
}
