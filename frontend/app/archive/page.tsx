import type { Metadata } from "next";
import ArticleBrowser, { type BrowseParams } from "../../components/article-browser";

export const metadata: Metadata = { title: "Archive" };
export default async function Archive({ searchParams }: { searchParams: Promise<BrowseParams> }) {
  return <ArticleBrowser params={await searchParams} basePath="/archive" />;
}
