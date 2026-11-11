import Link from "next/link";
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import ArticleComments from "../../../components/article-comments";
import { backendOrigin } from "../../../lib/public-api";
import type { Page } from "../../../lib/articles";
import type { PublicComment } from "../../../lib/comments";
import Markdown from "../../../components/markdown";
import ArticleContents from "../../../components/article-contents";
import { articleOutline } from "../../../lib/article-outline";
import { getPublicArticle } from "../../../lib/public-api";

type Props = { params: Promise<{ slug: string }> };

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const article = await getPublicArticle((await params).slug);
  if (!article) notFound();
  return { title: article.title, description: article.summary };
}

function date(value: string) {
  return new Intl.DateTimeFormat("en", { timeZone: "Asia/Shanghai", dateStyle: "medium", timeStyle: "short" }).format(new Date(value));
}

export default async function ArticlePage({ params }: Props) {
  const article = await getPublicArticle((await params).slug);
  if (!article) notFound();
  let commentPage: Page<PublicComment> | null = null;
  let missing = false;
  try {
    const response = await fetch(`${backendOrigin()}/api/v1/articles/${article.id}/comments`, { cache: "no-store", signal: AbortSignal.timeout(5000) });
    if (response.status === 404) missing = true;
    else if (response.ok) commentPage = await response.json();
  } catch { /* Keep article reading available when comment storage is unavailable. */ }
  const headings = articleOutline(article.markdown);
  return <article className="article-page" lang={article.language === "zh" ? "zh-CN" : "en"}>
    <header className="article-opening" id="article-top">
      <div className="article-opening-art" aria-hidden="true"><span /><span /><span /><span /></div>
      <div className="article-opening-copy">
        <Link className="article-back" href="/archive">← All writing</Link>
        <div className="article-labels">{article.category && <Link href={{ pathname: "/archive", query: { category: article.category.slug } }}>{article.category.name}</Link>}<span>{article.language === "zh" ? "中文" : "English"}</span></div>
        <h1>{article.title}</h1>
        {article.summary && <p className="article-standfirst">{article.summary}</p>}
        <div className="article-opening-byline"><span>iSaber</span>{article.tags.length > 0 && <nav aria-label="Article tags">{article.tags.map(tag => <Link key={tag.id} href={{ pathname: "/archive", query: { tag: tag.slug } }}>#{tag.name}</Link>)}</nav>}</div>
      </div>
      <dl className="article-dates">
        <div><dt>Written</dt><dd><time dateTime={article.written_at}>{article.written_at}</time></dd></div>
        <div><dt>Published</dt><dd><time dateTime={article.published_at}>{date(article.published_at)}</time></dd></div>
        <div><dt>Updated</dt><dd><time dateTime={article.updated_at}>{date(article.updated_at)}</time></dd></div>
      </dl>
    </header>
    <div className={`article-reading-layout${headings.some(heading => heading.depth <= 3) ? "" : " article-reading-layout--short"}`}>
      {headings.some(heading => heading.depth <= 3) && <ArticleContents headings={headings} />}
      <div className="article-reading-column">
        <div className="article-prose"><Markdown content={article.markdown} headings={headings} /></div>
        <div className="article-end"><Link href="/archive">← Back to Archive</Link><a href="#content">Back to top ↑</a></div>
        <ArticleComments articleId={article.id} enabled={!missing && article.comments_enabled} initialPage={commentPage} unavailable={!commentPage} />
      </div>
    </div>
    <footer className="article-footer"><Link className="site-brand" href="/">iSaber</Link><nav aria-label="Footer"><Link href="/archive">Archive ↗</Link><Link href="/about">About ↗</Link></nav></footer>
  </article>;
}
