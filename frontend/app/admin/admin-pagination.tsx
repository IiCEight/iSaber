import { pageTokens } from "../../lib/pagination";

export default function AdminPagination({ current, total, disabled, onChange }: {
  current: number; total: number; disabled: boolean; onChange: (page: number) => void;
}) {
  return <nav className="admin-pagination" aria-label="Admin article pages">
    <button className="button small admin-page-previous" type="button" disabled={disabled || current <= 1} onClick={() => onChange(current - 1)} aria-label="Previous page">← Previous</button>
    <ol className="admin-pagination-pages">{pageTokens(current, total).map(token => <li key={token}>{typeof token === "number"
      ? <button className="admin-page-number" type="button" aria-label={`Page ${token}`} aria-current={token === current ? "page" : undefined} disabled={disabled} onClick={() => { if (token !== current) onChange(token); }}>{token}</button>
      : <span className="admin-page-gap" aria-hidden="true">…</span>}</li>)}</ol>
    <button className="button small admin-page-next" type="button" disabled={disabled || current >= total} onClick={() => onChange(current + 1)} aria-label="Next page">Next →</button>
  </nav>;
}
