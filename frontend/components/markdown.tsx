// biome-ignore-all lint/performance/noImgElement: Markdown images use authorized direct delivery; an image optimizer must not cache private media or fetch arbitrary external URLs.
import ReactMarkdown from "react-markdown";
import type { ReactNode } from "react";
import type { PluggableList } from "unified";
import ArticleCode from "./article-code";
import { articleHeadingIds, type ArticleHeading } from "../lib/article-outline";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";
import rehypeKatex from "rehype-katex";
import rehypeHighlight from "rehype-highlight";

function ScrollRegion({ children, className, label }: { children: ReactNode; className: string; label: string }) {
  // biome-ignore lint/a11y/noNoninteractiveTabindex: Keyboard users need to focus horizontally overflowing code and tables to scroll them.
  return <section className={className} tabIndex={0} aria-label={label}>{children}</section>;
}

// Preview and published pages share this renderer. Raw HTML/MDX is never run.
export default function Markdown({ content, headings }: { content: string; headings?: ArticleHeading[] }) {
  const plugins: PluggableList = [remarkGfm, remarkMath];
  if (headings) plugins.push([articleHeadingIds, headings]);
  return (
    <div className="markdown">
      <ReactMarkdown
        skipHtml
        remarkPlugins={plugins}
        rehypePlugins={[
          [rehypeKatex, { trust: false, throwOnError: false, maxExpand: 1000, maxSize: 64 }],
          [rehypeHighlight, { ignoreMissing: true }],
        ]}
        components={{
          ...(headings ? {
            h1({ children, id }) { return <h2 id={id} tabIndex={-1}>{children}</h2>; },
            h2({ children, id }) { return <h2 id={id} tabIndex={-1}>{children}</h2>; },
            h3({ children, id }) { return <h3 id={id} tabIndex={-1}>{children}</h3>; },
            h4({ children, id }) { return <h4 id={id} tabIndex={-1}>{children}</h4>; },
            h5({ children, id }) { return <h5 id={id} tabIndex={-1}>{children}</h5>; },
            h6({ children, id }) { return <h6 id={id} tabIndex={-1}>{children}</h6>; },
            pre({ children, node }) {
              const code = node?.children.find(child => child.type === "element" && child.tagName === "code");
              const classes = code?.type === "element" ? String(code.properties.className ?? "") : "";
              const language = classes.match(/language-([\w-]+)/)?.[1] ?? "Code";
              return <ArticleCode language={language}><ScrollRegion className="article-code-scroll" label={`${language} code`}><pre>{children}</pre></ScrollRegion></ArticleCode>;
            },
            table({ children }) { return <ScrollRegion className="article-table-scroll" label="Article data table"><table>{children}</table></ScrollRegion>; },
          } satisfies import("react-markdown").Components : {}),
          img({ src, alt }) {
            if (typeof src === "string" && /^\/api\/v1\/articles\/[0-9a-f-]{36}\/assets\/[0-9a-f-]{36}$/.test(src)) {
              return <img src={src} alt={alt ?? ""} loading="lazy" referrerPolicy="no-referrer" />;
            }
            let remote: URL | null = null;
            try { if (typeof src === "string") remote = new URL(src); } catch { /* Show unavailable images without executing a URL. */ }
            if (remote?.protocol !== "https:" || !remote.hostname || remote.username || remote.password) {
              return <span className="image-warning">Image unavailable: {alt || "local image"}</span>;
            }
            return <span className="external-image"><small>External image · {remote.host}</small><img src={remote.href} alt={alt ?? ""} loading="lazy" referrerPolicy="no-referrer" /></span>;
          },
        }}
      >{content}</ReactMarkdown>
    </div>
  );
}
