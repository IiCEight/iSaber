import Link from "next/link";
import { pageHref, pageTokens } from "../lib/pagination";
import "./pagination.css";

export default function Pagination({ current, total, query, basePath = "/archive" }: {
  current: number; total: number; query: string; basePath?: string;
}) {
  if (total <= 1) return null;
  return <nav className="pagination" aria-label="Article pages">
    <div className="pagination-controls">
      {current > 1 ? <Link className="pagination-direction pagination-previous" href={pageHref(basePath, query, current - 1)} rel="prev" prefetch={false} aria-label="Previous page"><span aria-hidden="true">←</span><span className="pagination-direction-text">Previous</span></Link>
        : <span className="pagination-direction pagination-previous" aria-disabled="true"><span aria-hidden="true">←</span><span className="pagination-direction-text">Previous</span></span>}
      <ol className="pagination-pages">{pageTokens(current, total).map(token => <li key={token}>
        {typeof token !== "number" ? <span className="pagination-gap" aria-hidden="true">…</span>
          : token === current ? <span className="pagination-page" aria-current="page"><span className="pagination-sr-only">Page </span>{token}</span>
            : <Link className="pagination-page" href={pageHref(basePath, query, token)} prefetch={false} aria-label={`Page ${token}`}>{token}</Link>}
      </li>)}</ol>
      {current < total ? <Link className="pagination-direction pagination-next" href={pageHref(basePath, query, current + 1)} rel="next" prefetch={false} aria-label="Next page"><span className="pagination-direction-text">Next</span><span aria-hidden="true">→</span></Link>
        : <span className="pagination-direction pagination-next" aria-disabled="true"><span className="pagination-direction-text">Next</span><span aria-hidden="true">→</span></span>}
    </div>
  </nav>;
}
