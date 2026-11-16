import type { Metadata } from "next";
import { cookies } from "next/headers";
import SiteHeader from "../components/site-header";
import ThemeProvider from "../components/theme-provider";
import ReadingSession from "../components/reading-session";
import { getReadingAccess } from "../lib/reading-api";
import "./globals.css";
import "katex/dist/katex.min.css";
import "highlight.js/styles/github-dark.css";
import "./theme.css";

export async function generateMetadata(): Promise<Metadata> {
  const access = await getReadingAccess();
  return {
    title: { default: "iSaber", template: "%s | iSaber" },
    description: "Personal journals, technical articles, and selected work.",
    ...(access ? { robots: { index: false, follow: false } } : {}),
  };
}

// Establish a dynamic baseline: private content must never enter static pages.
// Caching can be introduced only with reviewed access and invalidation rules.
export const dynamic = "force-dynamic";

export default async function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  const access = await getReadingAccess();
  const theme = (await cookies()).get("isaber_theme")?.value === "dark" ? "dark" : "light";
  return (
    <html lang="en" data-theme={theme}>
      <body>
        <ThemeProvider initialTheme={theme}>
          <a className="skip-link" href="#content">Skip to content</a>
          <ReadingSession owner={!!access} expiresAt={access?.expiresAt ?? null}>
            <SiteHeader />
            <main id="content" className="app-main">{children}</main>
          </ReadingSession>
        </ThemeProvider>
      </body>
    </html>
  );
}
