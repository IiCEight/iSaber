import type { Metadata } from "next";
import Link from "next/link";
import AboutMotion, { AboutMotionControl } from "../../components/about-motion";
import "./about.css";

export const metadata: Metadata = { title: "About" };
const interests = [
  { title: "Notes", description: "Thoughts along the way" },
  { title: "Code", description: "Ideas explored through building" },
  { title: "Experiments", description: "Things made and tried" },
];
export default function About() {
  return <AboutMotion><div className="about-story">
    <div className="about-canvas" aria-hidden="true"><div className="warm-field" /><div className="orbit-layout"><div className="pointer-layer"><div className="orbit-rig">
      {Array.from({ length: 12 }, (_, i) => i * 30).map(degrees => <div className="petal-slot" key={degrees} style={{ left: `${50 + Math.cos(degrees * Math.PI / 180) * 30}%`, top: `${50 + Math.sin(degrees * Math.PI / 180) * 30}%` }}><span className="about-lens" /></div>)}
    </div></div></div></div>
    <section className="chapter chapter-hero" aria-labelledby="about-title"><div className="chapter-label"><span>iSaber / A personal space</span><span>English &amp; <span lang="zh-CN">中文</span></span></div><div className="hero-copy"><h1 id="about-title"><span className="title-line"><span>Curiosity,</span></span><span className="title-line"><span>in motion.</span></span></h1><p>Personal journals.<br />Technical notes.<br />Things I make.</p><a className="flat-link" href="#personal-note">Meet this little corner ↓</a></div><div className="hero-bottom"><span>01 / A place to begin</span><AboutMotionControl /><span>Scroll to connect the dots ↘</span></div></section>
    <section className="chapter chapter-note" id="personal-note" aria-labelledby="note-title"><p className="eyebrow">02 / A personal note</p><h2 id="note-title"><span>A space for</span><span>unfinished</span><span className="pink-word">ideas.</span></h2><div className="personal-copy"><span className="copy-label">Introduction · draft copy</span><p>A home for personal journals, technical articles, and things I make. Written in English and Chinese.</p><p className="chinese-note" lang="zh-CN">写下思考，记录探索。<br />让每一个想法，都有继续生长的空间。</p><p className="copy-note">Personal biography to be added.</p></div></section>
    <section className="chapter chapter-interests" id="directions" aria-labelledby="directions-title"><div className="directions-heading"><p className="eyebrow">03 / Follow a thread</p><h2 id="directions-title">Not one<br /><em>direction.</em></h2><p>Different subjects. One collection.<br />Browse the writing, wherever your curiosity leads.</p></div><nav className="interest-list" aria-label="Explore the writing">{interests.map((interest, i) => <Link className="interest-row" key={interest.title} href="/archive"><span className="row-number">{String(i + 1).padStart(2, "0")} /</span><span className="row-word">{interest.title}</span><span className="row-description">{interest.description}</span><span className="row-arrow" aria-hidden="true">↗</span></Link>)}</nav></section>
    <section className="chapter chapter-invitation" id="keep-exploring" aria-labelledby="invitation-title"><p className="eyebrow">04 / Still exploring</p><h2 id="invitation-title">Keep a little<br />space for<br /><span>wonder.</span></h2><div className="invitation-actions"><Link className="flat-link" href="/archive">Explore the writing ↗</Link><a href="#content">Back to the beginning ↑</a></div><div className="contact-note"><span>Contact / To be added</span><p>A public contact link will go here once provided.</p></div></section>
  </div><footer className="footer"><Link className="site-brand" href="/">iSaber</Link><nav aria-label="Footer"><Link href="/archive">Archive ↗</Link><Link href="/archive#archive-search-panel">Search ↗</Link></nav></footer></AboutMotion>;
}
