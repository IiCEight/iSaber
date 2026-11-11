import type { Nodes, Root } from "mdast";
import { toString as headingText } from "mdast-util-to-string";
import { unified } from "unified";
import remarkParse from "remark-parse";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";

export type ArticleHeading = { id: string; text: string; depth: number };
function walk(node: Nodes, visit: (node: Nodes) => void) {
  visit(node);
  if ("children" in node) for (const child of node.children) walk(child, visit);
}
// Share the same syntax as the Markdown renderer: fenced code, inline markup,
// Setext headings, math, and Chinese all need real parsing rather than a regex.
export function articleOutline(content: string): ArticleHeading[] {
  const tree = unified().use(remarkParse).use(remarkGfm).use(remarkMath).parse(content);
  const headings: ArticleHeading[] = [], used = new Set<string>();
  walk(tree, node => {
    if (node.type !== "heading") return;
    const text = headingText(node);
    const base = `article-heading-${text.normalize("NFKC").toLowerCase().replace(/[^\p{L}\p{N}]+/gu, "-").replace(/^-|-$/g, "") || "section"}`;
    let id = base, suffix = 2;
    while (used.has(id)) id = `${base}-${suffix++}`;
    used.add(id); headings.push({ id, text: text || "Untitled section", depth: node.depth });
  });
  return headings;
}
export function articleHeadingIds(headings: ArticleHeading[]) {
  return (tree: Root) => {
    let index = 0;
    walk(tree, node => {
      if (node.type !== "heading") return;
      const heading = headings[index++];
      if (heading) node.data = { ...node.data, hProperties: { ...node.data?.hProperties, id: heading.id } };
    });
  };
}
