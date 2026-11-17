"use client";
import Link from "next/link";
export default function CategoryError({ retry }: { error: unknown; retry: () => void }) {
 return <section className="archive-state"><h1>Category temporarily unavailable.</h1><p>Please try again shortly.</p><button type="button" onClick={retry}>Try again</button><Link href="/archive">Back to Archive ↗</Link></section>;
}
