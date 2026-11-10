// biome-ignore-all lint/performance/noImgElement: Markdown images use authorized direct delivery; an image optimizer must not cache private media or fetch arbitrary external URLs.
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";
import rehypeKatex from "rehype-katex";
import rehypeHighlight from "rehype-highlight";

// Preview and published pages share this renderer. Raw HTML/MDX is never run.
export default function Markdown({ content }: { content: string }) {
  return (
    <div className="markdown">
      <ReactMarkdown
        skipHtml
        remarkPlugins={[remarkGfm, remarkMath]}
        rehypePlugins={[
          [rehypeKatex, { trust: false, throwOnError: false, maxExpand: 1000, maxSize: 64 }],
          [rehypeHighlight, { ignoreMissing: true }],
        ]}
        components={{
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
