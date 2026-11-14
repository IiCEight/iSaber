import Link from "next/link";
import ArchiveFilters from "./archive-filters";
import Pagination from "./pagination";
import ArchiveSort from "./archive-sort";
import { defaultArticleSort } from "../lib/article-sort";
import type { Label, NumberedPage, PublicArticleSummary } from "../lib/articles";

export default function ArchiveView({ page, query, categories, tags, labelsUnavailable, error, searching, filtered }: {
  page: NumberedPage<PublicArticleSummary> | null; query: string; categories: Label[]; tags: Label[];
  labelsUnavailable: boolean; error: string | null; searching: boolean; filtered: boolean;
}) {
  const invalidQuery = error?.startsWith("Use ") || error?.startsWith("Check ");
  const offset = page ? (page.page - 1) * page.page_size : 0;
  const order = new URLSearchParams(query).get("sort") ?? defaultArticleSort;
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
      <div className="archive-results-note"><p role="status">{page ? page.total === 0 ? "0 articles" : `${offset + 1}–${offset + page.items.length} of ${page.total} article${page.total === 1 ? "" : "s"}` : "Collection unavailable"}</p><ArchiveSort query={query} searching={searching} /></div>
      {error ? <div className="archive-state"><p className="archive-eyebrow">{invalidQuery ? "A different thread" : "A brief interruption"}</p><h2>{invalidQuery ? "Check your search." : "The writing will be back."}</h2><p>{error}</p><a href={`/archive${query ? `?${query}` : ""}#archive-writing`}>Try again ↗</a><Link href="/archive#archive-writing">Clear conditions</Link></div>
        : page?.items.length === 0 ? <div className="archive-state"><p className="archive-eyebrow">{searching || filtered ? "A different thread" : "The first chapter"}</p><h2>{searching || filtered ? "No articles on this thread." : "The first chapter is on its way."}</h2><p>{searching ? "No public articles match this search." : filtered ? "No articles match these filters." : "There are no published articles yet. Come back for the first entry."}</p>{(searching || filtered) && <Link href="/archive#archive-writing">Clear all ↗</Link>}</div>
          : <ol className="archive-articles" start={offset + 1}>{page?.items.map((article, index) => <li className="archive-article-row" key={article.id}>
            <div className="archive-article-date"><span>{String(offset + index + 1).padStart(2, "0")} /</span><ArchiveArticleDate article={article} order={order} /></div>
            <div><h3 lang={article.language === "zh" ? "zh-CN" : "en"}><Link href={`/articles/${article.slug}`}>{article.title}</Link></h3>
              {article.summary && <p className="archive-summary" lang={article.language === "zh" ? "zh-CN" : "en"}>{article.summary}</p>}
              <div className="archive-article-meta">{article.category && <span className="archive-category">{article.category.name}</span>}{article.tags.map(tag => <span key={tag.id}>#{tag.name}</span>)}<span className="archive-language">{article.language === "zh" ? "中文" : "EN"}</span></div>
            </div><span className="archive-article-arrow" aria-hidden="true">↗</span>
          </li>)}</ol>}
      {page && !error && <Pagination current={page.page} total={page.total_pages} query={query} />}
      <div className="archive-collection-end"><p>{page && page.items.length > 0 && page.page === page.total_pages ? "You’ve reached the end of this collection." : ""}</p><a href="#content">Back to the top ↑</a></div>
    </section>
    <footer className="archive-footer"><Link className="site-brand" href="/">iSaber</Link><nav aria-label="Footer"><Link href="/">Home ↗</Link><Link href="/about">About ↗</Link></nav></footer>
  </div>;
}

function ArchiveArticleDate({ article, order }: { article: PublicArticleSummary; order: string }) {
  const written = order === "written" || order === "written_oldest";
  const updated = order === "updated" || order === "updated_oldest";
  const value = written ? article.written_at : updated ? article.updated_at : article.published_at;
  const label = written ? "Written date" : updated ? "Last updated" : "Published date";
  const date = new Date(written && value ? `${value}T00:00:00+08:00` : value);
  if (!value || Number.isNaN(date.getTime())) return <span>Not set</span>;
  const formatted = date.toLocaleDateString("en-GB", { timeZone: "Asia/Shanghai", day: "2-digit", month: "short", year: "numeric" });
  return <time dateTime={value} title={label}><span className="archive-date-label">{label}: </span>{formatted}</time>;
}
