import assert from "node:assert/strict";
import test from "node:test";
import { pageHref, pageTokens } from "../lib/pagination.ts";

test("pagination keeps the current page, endpoints, and a compact window", () => {
  assert.deepEqual(pageTokens(1, 2), [1, 2]);
  assert.deepEqual(pageTokens(1, 20), [1, 2, 3, 4, 5, "after", 20]);
  assert.deepEqual(pageTokens(10, 20), [1, "before", 9, 10, 11, "after", 20]);
  assert.deepEqual(pageTokens(20, 20), [1, "before", 16, 17, 18, 19, 20]);
  for (let current = 1; current <= 100; current++) {
    const tokens = pageTokens(current, 100);
    assert.ok(tokens.length <= 7);
    assert.ok(tokens.includes(current));
    assert.equal(tokens[0], 1);
    assert.equal(tokens.at(-1), 100);
    assert.equal(tokens.length, new Set(tokens).size);
  }
});

test("page links retain applied conditions and never expose a cursor", () => {
  assert.equal(pageHref("/archive", "", 2), "/archive?page=2#archive-writing");
  assert.equal(pageHref("/archive", "page=2", 1), "/archive#archive-writing");
  const href = pageHref("/archive", "q=Go+中文&category=notes&tag=go&language=zh&sort=oldest&cursor=old&page=2", 3);
  const url = new URL(href, "https://example.test");
  assert.equal(url.searchParams.get("q"), "Go 中文");
  assert.equal(url.searchParams.get("category"), "notes");
  assert.equal(url.searchParams.get("tag"), "go");
  assert.equal(url.searchParams.get("language"), "zh");
  assert.equal(url.searchParams.get("sort"), "oldest");
  assert.equal(url.searchParams.get("page"), "3");
  assert.equal(url.searchParams.has("cursor"), false);
});
