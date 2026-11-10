import Link from "next/link";
import { redirect } from "next/navigation";
import HomeMotion from "../components/home-motion";
import { backendOrigin } from "../lib/public-api";
import type { Page, PublicArticleSummary } from "../lib/articles";
import "./home.css";

type Params = Record<string, string | string[] | undefined>;

function Orbit({ variant }: { variant: string }) {
  return <div className={`home-orbit home-orbit--${variant}`}>
    {[0, 36, 72, 108, 144, 180, 216, 252, 288, 324].map(degrees => {
      const angle = degrees * Math.PI / 180;
      return <span key={degrees} style={{ left: `${(50 + Math.cos(angle) * 30).toFixed(3)}%`, top: `${(50 + Math.sin(angle) * 30).toFixed(3)}%`, rotate: `${degrees}deg` }} />;
    })}
  </div>;
}

export default async function Home({ searchParams }: { searchParams: Promise<Params> }) {
  const params = await searchParams;
  const legacyKeys = ["q", "category", "tag", "language", "cursor"];
  if (legacyKeys.some(key => params[key] !== undefined)) {
    const query = new URLSearchParams();
    for (const key of legacyKeys) {
      const values = params[key];
      if (Array.isArray(values)) for (const value of values) query.append(key, value);
      else if (values !== undefined) query.set(key, values);
    }
    redirect(`${params.q !== undefined ? "/search" : "/archive"}?${query}`);
  }
  let page: Page<PublicArticleSummary> | null = null;
  try {
    const response = await fetch(`${backendOrigin()}/api/v1/articles?limit=4`, { cache: "no-store", signal: AbortSignal.timeout(5000) });
    if (response.ok) page = await response.json();
  } catch { /* Keep identity and navigation available during API outages. */ }
  return <HomeMotion>
    <section className="home-story" aria-label="Explore iSaber">
      <div className="home-scene">
        <div className="home-scene-background" aria-hidden="true"><div className="home-warm-pane" /><div className="home-ink-pane" /></div>
        <div className="home-graphic" aria-hidden="true"><Orbit variant="one" /><Orbit variant="two" /><Orbit variant="three" /></div>
        <section className="home-hero" aria-labelledby="home-title">
          <p className="home-eyebrow">A personal space<br />English &amp; <span lang="zh-CN">中文</span></p>
          <div className="home-hero-type">
            <h1 className="home-title" id="home-title"><span>iSaber</span><span>notes &amp; ideas.</span></h1>
            <a className="home-writing-link" href="#recent-articles">Explore the writing <span aria-hidden="true">↓</span></a>
          </div>
        </section>
        <section className="home-story-intro" aria-labelledby="home-intro-title">
          <h2 id="home-intro-title"><span>Keep</span><span>exploring.</span></h2>
          <div className="home-intro-card">
            <p className="home-panel-label">A journal of curiosity</p>
            <p>A place for personal notes, technical explorations, and things I make. Written in English and Chinese.</p>
            <Link href="/about">More about me <span aria-hidden="true">↗</span></Link>
          </div>
        </section>
        <section className="home-story-invitation" aria-labelledby="home-invitation-title">
          <p className="home-panel-label">Things learned. Things made.</p>
          <h2 id="home-invitation-title">Stay curious.</h2>
          <a className="home-writing-link" href="#recent-articles">Read the latest <span aria-hidden="true">↓</span></a>
          <span className="home-scroll-cue" aria-hidden="true">Keep scrolling ↓</span>
        </section>
      </div>
      <div className="home-story-track" aria-hidden="true" />
      <div className="home-content">
      <section aria-labelledby="recent-articles">
        <div className="home-section-heading"><h2 id="recent-articles" tabIndex={-1}>Recent writing</h2><Link href="/archive">View archive <span aria-hidden="true">↗</span></Link></div>
        {!page ? <div className="home-state"><h3>Articles are temporarily unavailable.</h3><p>Please try again shortly.</p><a href="/#recent-articles">Try again</a></div>
          : page.items.length === 0 ? <div className="home-state"><h3>The first chapter is on its way.</h3><p>New public articles will appear here.</p></div>
          : <ol className="home-articles">{page.items.slice(0, 4).map((article, index) => <li key={article.id} data-home-article>
            <span className="home-article-number" aria-hidden="true">{String(index + 1).padStart(2, "0")}</span>
            <article lang={article.language === "zh" ? "zh-CN" : "en"}>
              <h3><Link href={`/articles/${article.slug}`}>{article.title}<span className="home-article-arrow" aria-hidden="true">↗</span></Link></h3>
              {article.summary && <p>{article.summary}</p>}
              <div className="home-article-meta">
                {article.category && <Link href={{ pathname: "/archive", query: { category: article.category.slug } }}>{article.category.name}</Link>}
                <span>{article.language === "zh" ? "中文" : "English"}</span>
                <time dateTime={article.published_at} lang="en">{new Date(article.published_at).toLocaleDateString("en-GB", { timeZone: "Asia/Shanghai", day: "2-digit", month: "short", year: "numeric" })}</time>
              </div>
            </article>
          </li>)}</ol>}
      </section>
      <section className="home-explore" aria-label="Explore more writing"><p>There is more to explore.</p><Link href="/archive">Browse all articles <span aria-hidden="true">↗</span></Link></section>
      <footer className="home-footer"><a href="#content">iSaber <span aria-hidden="true">↑</span></a><nav aria-label="Footer"><Link href="/archive">Archive</Link><Link href="/about">About</Link></nav></footer>
      </div>
    </section>
  </HomeMotion>;
}
