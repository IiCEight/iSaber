import type { Metadata } from "next";
import Link from "next/link";

export const metadata: Metadata = { title: "About" };
export default function About() {
  return <section className="about-page">
    <p>iSaber · A personal space</p>
    <h1>Words, ideas, and discoveries.</h1>
    <p>A home for personal journals, technical articles, and things I make. Written in English and Chinese.</p>
    <p>Explore the <Link href="/archive">archive</Link> to find articles by category, tag, or language.</p>
  </section>;
}
