import Link from "next/link";
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import Markdown from "../../../components/markdown";
import { getPublicArticle } from "../../../lib/public-api";

type Props = { params: Promise<{ slug: string }> };

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const article = await getPublicArticle((await params).slug);
  return article ? { title: article.title, description: article.summary } : { title: "Article not found", robots: { index: false } };
}

function date(value: string) {
  return new Intl.DateTimeFormat("en", { timeZone: "Asia/Shanghai", dateStyle: "medium", timeStyle: "short" }).format(new Date(value));
}

export default async function ArticlePage({ params }: Props) {
  const article = await getPublicArticle((await params).slug);
  if (!article) notFound();
  return <article lang={article.language === "zh" ? "zh-CN" : "en"}>
    <h1>{article.title}</h1>
    <dl className="article-dates">
      <div><dt>Written</dt><dd>{article.written_at}</dd></div>
      <div><dt>Published</dt><dd><time dateTime={article.published_at}>{date(article.published_at)}</time></dd></div>
      <div><dt>Updated</dt><dd><time dateTime={article.updated_at}>{date(article.updated_at)}</time></dd></div>
    </dl>
    {(article.category || article.tags.length > 0) && <nav aria-label="Article categories and tags">
      {article.category && <p>Category: <Link href={{ pathname: "/", query: { category: article.category.slug } }}>{article.category.name}</Link></p>}
      {article.tags.length > 0 && <ul>{article.tags.map((tag) => <li key={tag.id}><Link href={{ pathname: "/", query: { tag: tag.slug } }}>{tag.name}</Link></li>)}</ul>}
    </nav>}
    <Markdown content={article.markdown} />
  </article>;
}
