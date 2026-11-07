import type { Metadata } from "next";
import Link from "next/link";
import "./globals.css";
import "katex/dist/katex.min.css";
import "highlight.js/styles/github-dark.css";

export const metadata: Metadata = {
  title: { default: "iSaber", template: "%s | iSaber" },
  description: "Personal journals, technical articles, and selected work.",
};

// Establish a dynamic baseline: private content must never enter static pages.
// Caching can be introduced only with reviewed access and invalidation rules.
export const dynamic = "force-dynamic";

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body>
        <a className="skip-link" href="#content">Skip to content</a>
        <header><Link href="/" aria-label="iSaber home">iSaber</Link></header>
        <main id="content">{children}</main>
      </body>
    </html>
  );
}
