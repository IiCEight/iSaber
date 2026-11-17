import Link from "next/link";
import type { Category } from "../lib/articles";
import CategoryMotion from "./category-motion";

export default function CategoryMasthead({ category }: { category: Category }) {
 return <section className="category-masthead" aria-labelledby="category-title">
  <div className="category-masthead-copy">
   <nav className="category-breadcrumb" aria-label="Breadcrumb"><Link href="/archive">Archive</Link><span aria-hidden="true">/</span><span aria-current="page">{category.name}</span></nav>
   <h1 id="category-title">{category.name}.</h1>
   <div className="category-description">{category.description_en && <p lang="en">{category.description_en}</p>}{category.description_zh && <p lang="zh-CN">{category.description_zh}</p>}</div>
  </div>
  <CategoryMotion />
  <div className="category-masthead-foot"><span>{category.total} {category.total === 1 ? "article" : "articles"} in this thread</span><span>English &amp; <span lang="zh-CN">中文</span></span></div>
 </section>;
}
