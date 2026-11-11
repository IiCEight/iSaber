"use client";

import Link from "next/link";

export default function ArticleError({ retry }: { error: unknown; retry: () => void }) {
  return <div className="article-page"><section className="article-unavailable" aria-labelledby="article-error-title">
    <p>Reading can wait a moment.</p><h1 id="article-error-title">Article temporarily unavailable.</h1>
    <p>Please try again shortly, or return to the collection.</p>
    <div><button type="button" onClick={retry}>Try again</button><Link href="/archive">Back to Archive ↗</Link></div>
  </section></div>;
}
