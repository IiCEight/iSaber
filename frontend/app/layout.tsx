import type { Metadata } from "next";
import SiteHeader from "../components/site-header";
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
        <SiteHeader />
        <main id="content" className="app-main">{children}</main>
      </body>
    </html>
  );
}
