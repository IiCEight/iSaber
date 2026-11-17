import type { Metadata } from "next";
import { cache } from "react";
import { notFound } from "next/navigation";
import ArticleBrowser, { type BrowseParams } from "../../../components/article-browser";
import { getReadingAccess, readingFetch } from "../../../lib/reading-api";
import type { Category } from "../../../lib/articles";
import "../../archive/archive.css";
import "../category.css";

const getCategory = cache(async (slug: string): Promise<Category | null> => {
 if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(slug) || slug.length > 160) return null;
 const response = await readingFetch(`categories/${encodeURIComponent(slug)}`);
 if (response.status === 404) return null;
 if (!response.ok) throw new Error("Category temporarily unavailable");
 return response.json();
});
export async function generateMetadata({ params }: { params: Promise<{slug: string}> }): Promise<Metadata> {
 const category = await getCategory((await params).slug);
 return { title: category?.name ?? "Category", robots: await getReadingAccess() ? { index: false, follow: false } : undefined };
}
export default async function CategoryPage({ params, searchParams }: { params: Promise<{slug: string}>; searchParams: Promise<BrowseParams> }) {
 const category = await getCategory((await params).slug);
 if (!category) notFound();
 return <ArticleBrowser params={await searchParams} basePath={`/categories/${category.slug}`} category={category} />;
}
