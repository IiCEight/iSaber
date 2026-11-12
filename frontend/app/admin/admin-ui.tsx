import type { ReactNode } from "react";

const paths = {
  articles: "M4 4h16v16H4z M8 8h8 M8 12h8 M8 16h5",
  import: "M12 16V3 M7 8l5-5 5 5 M4 15v6h16v-6",
  labels: "M3 4h8l10 10-7 7L3 10z M7 8h.01",
  comments: "M4 4h16v13H9l-5 4z M8 8h8 M8 12h5",
  info: "M12 8h.01 M12 11v6 M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0",
  file: "M6 3h8l4 4v14H6z M14 3v5h4 M9 12h6 M9 16h6",
};
export function AdminIcon({ name }: { name: keyof typeof paths }) {
  return <svg className="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d={paths[name]} /></svg>;
}
export function AdminBadge({ children, tone = "" }: { children: ReactNode; tone?: string }) {
  return <span className={`badge ${tone}`}>{children}</span>;
}
export function AdminTable({ children, label }: { children: ReactNode; label: string }) {
  // biome-ignore lint/a11y/noNoninteractiveTabindex: Wide management tables need a named keyboard-focusable horizontal scroll region.
  return <section className="table-scroll" aria-label={label} tabIndex={0}>{children}</section>;
}
export function AdminNote({ children }: { children: ReactNode }) {
  return <div className="note"><AdminIcon name="info" /><p>{children}</p></div>;
}
export function adminDate(value: string | null) {
  return value ? new Intl.DateTimeFormat("en-GB", { timeZone: "Asia/Shanghai", day: "2-digit", month: "short", year: "numeric" }).format(new Date(value)) : "Not published";
}
