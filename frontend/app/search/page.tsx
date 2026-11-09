import type { Metadata } from "next";
import ArticleBrowser, { type BrowseParams } from "../../components/article-browser";

export const metadata: Metadata = { title: "Search", robots: { index: false } };
export default async function Search({ searchParams }: { searchParams: Promise<BrowseParams> }) {
  const params = await searchParams;
  return <ArticleBrowser params={{ ...params, q: params.q ?? "" }} basePath="/search" />;
}
