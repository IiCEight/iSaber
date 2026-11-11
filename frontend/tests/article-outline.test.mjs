import assert from "node:assert/strict";
import test from "node:test";
import { articleOutline, articleHeadingIds } from "../lib/article-outline.ts";
import { unified } from "unified";
import remarkParse from "remark-parse";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";

test("contents parse real headings while ignoring code and raw HTML", () => {
  const content = '# Title\n\nSetext *heading*\n---\n\n```md\n## Not a heading\n```\n\n<h2>Raw HTML</h2>\n\n### 中文与 `code`\n';
  const headings = articleOutline(content);
  assert.deepEqual(headings.map(({ text, depth }) => ({ text, depth })), [
    { text: "Title", depth: 1 }, { text: "Setext heading", depth: 2 }, { text: "中文与 code", depth: 3 },
  ]);
  assert.equal(headings[2].id, "article-heading-中文与-code");
});
test("duplicate headings and suffix-like titles get unique namespaced anchors", () => {
  const headings = articleOutline('## Comments title\n## Same\n## Same\n## Same-2\n## !!!\n## !!!');
  assert.equal(new Set(headings.map(heading => heading.id)).size, headings.length);
  assert.deepEqual(headings.slice(1).map(heading => heading.id), ["article-heading-same", "article-heading-same-2", "article-heading-same-2-2", "article-heading-section", "article-heading-section-2"]);
});
test("the rendering plugin assigns the exact anchors exposed to navigation", () => {
  const source = '## **Hello**\n\n> ### Nested\n\n## $x^2$';
  const headings = articleOutline(source);
  const tree = unified().use(remarkParse).use(remarkGfm).use(remarkMath).parse(source);
  articleHeadingIds(headings)(tree);
  const ids = [];
  function visit(node) { if (node.type === "heading") ids.push(node.data.hProperties.id); if (node.children) node.children.forEach(visit); }
  visit(tree);
  assert.deepEqual(ids, headings.map(heading => heading.id));
  assert.deepEqual(articleOutline('No headings.\n\n```\n# Still code\n```'), []);
});
