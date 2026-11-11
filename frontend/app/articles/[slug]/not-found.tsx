import Link from "next/link";

export default function ArticleNotFound() {
  return <div className="article-page"><section className="article-unavailable" aria-labelledby="article-missing-title">
    <p>A different thread.</p><h1 id="article-missing-title">Article not found.</h1>
    <p>This article isn’t available at this address.</p><Link href="/archive">Back to Archive ↗</Link>
  </section></div>;
}
