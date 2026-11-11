import Link from "next/link";
import ArchiveFilters from "./archive-filters";
import type { Label, Page, PublicArticleSummary } from "../lib/articles";

export default function ArchiveView({ page, query, categories, tags, labelsUnavailable, error, searching, filtered, nextHref }: {
  page: Page<PublicArticleSummary> | null; query: string; categories: Label[]; tags: Label[];
  labelsUnavailable: boolean; error: string | null; searching: boolean; filtered: boolean; nextHref: string | null;
}) {
  const invalidQuery = error?.startsWith("Use ") || error?.startsWith("Check ");
  return <div className="archive-page">
    <section className="archive-masthead" aria-labelledby="archive-title">
      <div className="archive-masthead-copy"><p className="archive-eyebrow">01 / The collection</p><h1 id="archive-title">Archive.</h1><p>Notes, explorations, and things made along the way.</p></div>
      <div className="archive-lens-art" aria-hidden="true">
        {[0, 36, 72, 108, 144, 180, 216, 252, 288, 324].map(degrees => {
          const angle = degrees * Math.PI / 180;
          return <span key={degrees} style={{ left: `${50 + Math.cos(angle) * 30}%`, top: `${50 + Math.sin(angle) * 30}%` }} />;
        })}
        <div className="archive-lens-label"><span>↘</span>Follow a thread</div>
      </div>
      <div className="archive-masthead-foot"><span>Personal notes &amp; technical writing</span><span>English &amp; <span lang="zh-CN">中文</span></span></div>
    </section>
    <section className="archive-index" aria-label="Article collection">
      <ArchiveFilters query={query} categories={categories} tags={tags} labelsUnavailable={labelsUnavailable} />
      <div className="archive-results-note"><p role="status">{page ? `${page.items.length} article${page.items.length === 1 ? "" : "s"} on this page` : "Collection unavailable"}</p><p>{searching ? "Best match first ↓" : "Published date · Newest first ↓"}</p></div>
      {error ? <div className="archive-state"><p className="archive-eyebrow">{invalidQuery ? "A different thread" : "A brief interruption"}</p><h2>{invalidQuery ? "Check your search." : "The writing will be back."}</h2><p>{error}</p><a href={`/archive${query ? `?${query}` : ""}#archive-writing`}>Try again ↗</a><Link href="/archive#archive-writing">Clear conditions</Link></div>
        : page?.items.length === 0 ? <div className="archive-state"><p className="archive-eyebrow">{searching || filtered ? "A different thread" : "The first chapter"}</p><h2>{searching || filtered ? "No articles on this thread." : "The first chapter is on its way."}</h2><p>{searching ? "No public articles match this search." : filtered ? "No articles match these filters." : "There are no published articles yet. Come back for the first entry."}</p>{(searching || filtered) && <Link href="/archive#archive-writing">Clear all ↗</Link>}</div>
          : <ol className="archive-articles">{page?.items.map((article, index) => <li className="archive-article-row" key={article.id}>
            <div className="archive-article-date"><span>{String(index + 1).padStart(2, "0")} /</span><time dateTime={article.published_at}>{new Date(article.published_at).toLocaleDateString("en-GB", { timeZone: "Asia/Shanghai", day: "2-digit", month: "short", year: "numeric" })}</time></div>
            <div><h3 lang={article.language === "zh" ? "zh-CN" : "en"}><Link href={`/articles/${article.slug}`}>{article.title}</Link></h3>
              {article.summary && <p className="archive-summary" lang={article.language === "zh" ? "zh-CN" : "en"}>{article.summary}</p>}
              <div className="archive-article-meta">{article.category && <span className="archive-category">{article.category.name}</span>}{article.tags.map(tag => <span key={tag.id}>#{tag.name}</span>)}<span className="archive-language">{article.language === "zh" ? "中文" : "EN"}</span></div>
            </div><span className="archive-article-arrow" aria-hidden="true">↗</span>
          </li>)}</ol>}
      <div className="archive-collection-end">{nextHref ? <Link className="archive-more" href={nextHref}>More articles ↗</Link> : <p>{page && page.items.length > 0 ? "You’ve reached the end of this collection." : "There is more to explore."}</p>}<a href="#content">Back to the top ↑</a></div>
    </section>
    <footer className="archive-footer"><Link className="site-brand" href="/">iSaber</Link><nav aria-label="Footer"><Link href="/">Home ↗</Link><Link href="/about">About ↗</Link></nav></footer>
  </div>;
}
