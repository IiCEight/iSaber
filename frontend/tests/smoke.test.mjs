import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { once } from "node:events";
import { createServer } from "node:http";
import { createRequire } from "node:module";
import { setTimeout as delay } from "node:timers/promises";
import test from "node:test";
import { fileURLToPath } from "node:url";

const require = createRequire(import.meta.url);
const root = fileURLToPath(new URL("../", import.meta.url));
const origin = "http://127.0.0.1:4317";

test("production frontend handles unavailable and older backend capabilities", async (t) => {
  let listMode = "unavailable";
  const publicRequests = [];
  let readerAuthenticated = true;
  const readerCookie = "isaber_dev_session=reader-fixture";
  const sampleArticles = Array.from({ length: 6 }, (_, index) => ({
    id: `11111111-1111-4111-8111-${String(index + 1).padStart(12, "0")}`, slug: `public-article-${index}`,
    title: index === 1 ? "中文标题与阅读" : `Public article ${index}`, summary: `Summary ${index}`, language: index === 1 ? "zh" : "en",
    written_at: "2026-11-07", published_at: "2026-11-07T08:00:00Z", updated_at: "2026-11-07T08:00:00Z",
    category: { id: "label", name: "Notes", slug: "notes" }, tags: [],
  }));
  const manyArticles = Array.from({ length: 35 }, (_, index) => ({
    ...sampleArticles[index % sampleArticles.length],
    id: `22222222-2222-4222-8222-${String(index + 1).padStart(12, "0")}`,
    slug: `numbered-article-${index + 1}`, title: `Numbered article ${index + 1}`,
  }));
  const privateArticle = { ...sampleArticles[0], id: "33333333-3333-4333-8333-333333333333", slug: "private-fixture", title: "Private fixture title", summary: "Private fixture summary", visibility: "private", markdown: "# Private fixture title\n\nowner-only-body-marker", comments_enabled: true };
  const olderAPI = createServer((request, response) => {
    const url = new URL(request.url, "http://fixture.invalid");
    publicRequests.push({ path: url.pathname, query: url.searchParams, cookie: request.headers.cookie });
    if (url.pathname === "/api/v1/auth/session") {
      const authenticated = (request.headers.cookie === "fixture-owner=session" || (readerAuthenticated && request.headers.cookie === readerCookie));
      response.writeHead(authenticated ? 200 : 401, { "Content-Type": "application/json", "Cache-Control": "no-store" });
      response.end(JSON.stringify(authenticated ? { owner: { id: "owner-fixture", username: "private-owner-fixture" }, csrf_token: "private-csrf-fixture", expires_at: "2027-01-01T00:00:00Z" } : { code: "unauthorized" }));
    } else if (url.pathname.startsWith("/api/v1/admin/reading/")) {
      if (!readerAuthenticated || request.headers.cookie !== readerCookie) { response.writeHead(401, { "Content-Type": "application/problem+json" }); response.end(JSON.stringify({ code: "unauthorized" })); return; }
      response.writeHead(200, { "Content-Type": "application/json", "Cache-Control": "private, no-store", "X-Robots-Tag": "noindex, nofollow" });
      if (url.pathname.endsWith("/private-fixture")) response.end(JSON.stringify(privateArticle));
      else if (url.pathname.endsWith("/categories") || url.pathname.endsWith("/tags")) response.end(JSON.stringify({ items: [{ id: "private-label", name: "Private category fixture", slug: "private-category" }], next_cursor: null }));
      else {
        const source = url.pathname.endsWith("/search") ? [privateArticle] : [privateArticle, ...sampleArticles.map(article => ({ ...article, visibility: "public" }))];
        const size = Number(url.searchParams.get("limit") ?? 20), page = Number(url.searchParams.get("page") ?? 1);
        const { markdown: _markdown, comments_enabled: _comments, ...summary } = privateArticle;
        const items = source.slice((page - 1) * size, page * size).map(article => article.visibility === "private" ? summary : article);
        response.end(JSON.stringify({ items, next_cursor: null, ...(url.searchParams.has("page") ? { page, page_size: size, total: source.length, total_pages: Math.max(1, Math.ceil(source.length / size)) } : {}) }));
      }
    } else if ((url.pathname === "/api/v1/articles" || url.pathname === "/api/v1/articles/search") && listMode !== "unavailable") {
      response.writeHead(200, { "Content-Type": "application/json", "Cache-Control": "no-store" });
      const source = listMode === "empty" ? [] : listMode === "paginated" ? manyArticles : sampleArticles;
      const items = url.searchParams.get("sort") === "oldest" ? [...source].reverse() : source;
      if (url.searchParams.has("page")) {
        const pageSize = Number(url.searchParams.get("limit") ?? "20"), totalPages = Math.max(1, Math.ceil(items.length / pageSize));
        const page = Math.min(Number(url.searchParams.get("page")), totalPages);
        response.end(JSON.stringify({ items: items.slice((page - 1) * pageSize, page * pageSize), next_cursor: null, page, page_size: pageSize, total: items.length, total_pages: totalPages }));
      } else response.end(JSON.stringify({ items, next_cursor: listMode === "empty" ? null : "fixture-next" }));
    } else if (request.url === "/api/v1/articles/legacy-article") {
      response.writeHead(200, { "Content-Type": "application/json", "Cache-Control": "no-store" });
      response.end(JSON.stringify({ id: "11111111-1111-4111-8111-111111111111", slug: "legacy-article", title: "Legacy article", summary: "A readable existing article.", language: "en", written_at: "2026-11-07", published_at: "2026-11-07T08:00:00Z", updated_at: "2026-11-07T08:00:00Z", category: null, tags: [], markdown: "Existing article body.", comments_enabled: true }));
    } else { response.writeHead(404, { "Content-Type": "application/problem+json" }); response.end(JSON.stringify({ status: 404, code: "not_found" })); }
  });
  olderAPI.listen(0, "127.0.0.1"); await once(olderAPI, "listening");
  t.after(() => new Promise((resolve, reject) => olderAPI.close((error) => error ? reject(error) : resolve())));
  const backend = `http://127.0.0.1:${olderAPI.address().port}`;
  const server = spawn(process.execPath, [require.resolve("next/dist/bin/next"), "start", "--hostname", "127.0.0.1", "--port", "4317"], {
    cwd: root,
    env: { ...process.env, NODE_ENV: "production", NEXT_TELEMETRY_DISABLED: "1", BACKEND_ORIGIN: backend },
    stdio: ["ignore", "pipe", "pipe"],
  });
  let logs = "";
  let spawnError;
  server.on("error", (error) => { spawnError = error; });
  server.stdout.on("data", (chunk) => { logs += chunk.toString(); });
  server.stderr.on("data", (chunk) => { logs += chunk.toString(); });
  t.after(async () => {
    if (server.exitCode !== null || server.signalCode !== null || spawnError) return;
    const exited = once(server, "exit");
    server.kill("SIGTERM");
    const forceStop = setTimeout(() => server.kill("SIGKILL"), 5000);
    try { await exited; } finally { clearTimeout(forceStop); }
  });

  let ready = false;
  for (let attempt = 0; attempt < 200; attempt++) {
    assert.ifError(spawnError);
    assert.equal(server.exitCode, null, `Frontend exited before readiness:\n${logs}`);
    try {
      const response = await fetch(`${origin}/healthz`, { signal: AbortSignal.timeout(1000) });
      if (response.ok) { ready = true; break; }
    } catch { /* Give the production server time to start. */ }
    await delay(100);
  }
  assert.ok(ready, `Frontend did not become ready:\n${logs}`);

  await t.test("home page is server rendered and not cached", async () => {
    const response = await fetch(origin);
    assert.equal(response.status, 200);
    assert.match(response.headers.get("cache-control") ?? "", /no-store/);
    const html = await response.text();
    assert.match(html, /id="home-title"/);
    assert.match(html, /notes &amp; ideas/);
    assert.match(html, /Articles are temporarily unavailable/);
    assert.match(html, /Skip to content/);
  });
  await t.test("home renders only four public summaries without forwarding reader cookies", async () => {
    listMode = "populated";
    publicRequests.length = 0;
    const html = await (await fetch(origin, { headers: { Cookie: "test-private-session=fixture" } })).text();
    assert.match(html, /Public article 0/);
    assert.match(html, /中文标题与阅读/);
    assert.doesNotMatch(html, /Public article 4/);
    assert.match(html, /href="\/archive\?category=notes"/);
    const listing = publicRequests.find(request => request.path === "/api/v1/articles");
    assert.equal(listing.query.get("limit"), "4");
    assert.equal(listing.cookie, undefined);
    assert.equal(publicRequests.some(request => request.path.startsWith("/api/v1/admin")), false);
  });
  await t.test("home handles a successful empty response separately from an outage", async () => {
    listMode = "empty";
    const html = await (await fetch(origin)).text();
    assert.match(html, /The first chapter is on its way/);
    assert.doesNotMatch(html, /Articles are temporarily unavailable/);
    listMode = "populated";
  });
  await t.test("archive preserves filters and pagination", async () => {
    listMode = "paginated";
    const html = await (await fetch(`${origin}/archive?category=notes&tag=go&language=zh`)).text();
    assert.match(html, /id="archive-title">Archive\.<\/h1>/);
    assert.ok(html.includes('/archive?category=notes&amp;tag=go&amp;language=zh&amp;page=2#archive-writing'));
    assert.match(html, /action="\/archive#archive-writing"/);
    assert.match(html, /Applied search conditions/);
    assert.match(html, /Remove Category: notes|Remove Category: Notes/);
    listMode = "populated";
  });
  await t.test("Archive search retains applied conditions and drops pagination when removing one", async () => {
    publicRequests.length = 0;
    listMode = "paginated";
    const html = await (await fetch(`${origin}/archive?q=Go&category=notes&language=en&page=2`, { headers: { Cookie: "reader-session=fixture" } })).text();
    assert.match(html, /Remove Search: Go/);
    assert.ok(html.includes('/archive?language=en&amp;q=Go#archive-writing'));
    const request = publicRequests.find(request => request.path === "/api/v1/articles/search");
    assert.equal(request.query.get("q"), "Go");
    assert.equal(request.query.get("category"), "notes");
    assert.equal(request.query.get("page"), "2");
    assert.equal(request.query.get("limit"), "10");
    assert.equal(request.cookie, undefined);
    listMode = "populated";
  });
  await t.test("Archive numbered pages expose the full count, continuous numbering, and clean links", async () => {
    listMode = "paginated";
    const first = await (await fetch(`${origin}/archive`)).text();
    assert.match(first, /1–10 of 35 articles/);
    assert.match(first, /aria-current="true" aria-label="Sort by written date"/);
    assert.match(first, /title="Written date"/);
    assert.equal((first.match(/class="archive-article-row"/g) ?? []).length, 10);
    assert.match(first, /aria-label="Article pages"/);
    assert.match(first, /aria-disabled="true"/);
    assert.ok(first.includes('/archive?page=2#archive-writing'));
    const second = await (await fetch(`${origin}/archive?page=2`)).text();
    assert.match(second, /11–20 of 35 articles/);
    assert.match(second, /class="archive-articles" start="11"/);
    assert.equal((second.match(/class="archive-article-row"/g) ?? []).length, 10);
    assert.match(second, /Numbered article 20/);
    assert.doesNotMatch(second, /More articles|cursor=/);
    assert.match(second, /rel="prev"/);
    assert.match(second, /rel="next"/);
    const last = await (await fetch(`${origin}/archive?page=4`)).text();
    assert.match(last, /31–35 of 35 articles/);
    assert.equal((last.match(/class="archive-article-row"/g) ?? []).length, 5);
    assert.match(last, /Numbered article 35/);
    assert.doesNotMatch(last, /rel="next"/);
    listMode = "populated";
  });
  await t.test("Archive sorts the collection and preserves ordering in pagination and filter forms", async () => {
    listMode = "paginated";
    publicRequests.length = 0;
    const html = await (await fetch(`${origin}/archive?q=Go&language=en&sort=oldest&page=2`)).text();
    assert.match(html, /Sort articles/);
    assert.match(html, /aria-current="true" aria-label="Sort by published date"/);
    assert.match(html, /Switch to newest first/);
    assert.match(html, /Numbered article 25/);
    assert.match(html, /type="hidden" name="sort" value="oldest"/);
    const nextLinks = [...html.matchAll(/href="([^"]+)"/g)].map(match => new URL(match[1].replaceAll("&amp;", "&"), origin));
    assert.ok(nextLinks.some(url => url.pathname === "/archive" && url.searchParams.get("page") === "3" && url.searchParams.get("sort") === "oldest" && url.searchParams.get("q") === "Go"));
    const request = publicRequests.find(request => request.path === "/api/v1/articles/search");
    assert.equal(request.query.get("sort"), "oldest");
    assert.equal(request.query.get("page"), "2");
    assert.equal(request.query.get("limit"), "10");
    for (const [order, dateLabel] of [["written", "Written date"], ["written_oldest", "Written date"], ["updated", "Last updated"], ["updated_oldest", "Last updated"], ["newest", "Published date"]]) {
      publicRequests.length = 0;
      const sorted = await (await fetch(`${origin}/archive?sort=${order}`)).text();
      const field = order.startsWith("written") ? "written date" : order.startsWith("updated") ? "last updated" : "published date";
      assert.ok(sorted.includes(`aria-current="true" aria-label="Sort by ${field}"`));
      assert.doesNotMatch(sorted, /<select[^>]+name="sort"/);
      assert.match(sorted, new RegExp(`title="${dateLabel}"`));
      assert.equal(publicRequests.find(request => request.path === "/api/v1/articles").query.get("sort"), order);
    }
    for (const query of ["sort=unknown", "sort=oldest&sort=newest"]) {
      assert.match(await (await fetch(`${origin}/archive?${query}`)).text(), /Use a valid article order/);
    }
    listMode = "populated";
  });
  await t.test("Archive rejects invalid page numbers and resets legacy cursor links", async () => {
    for (const query of ["page=0", "page=-1", "page=1.5", "page=1&page=2"]) {
      const html = await (await fetch(`${origin}/archive?${query}`)).text();
      assert.match(html, /Use a positive whole page number/);
    }
    const legacy = await fetch(`${origin}/archive?category=notes&cursor=fixture-next`, { redirect: "manual" });
    assert.equal(legacy.status, 307);
    assert.equal(legacy.headers.get("location"), "/archive?category=notes#archive-writing");
  });
  await t.test("a blank Archive search field still browses the selected category", async () => {
    publicRequests.length = 0;
    const html = await (await fetch(`${origin}/archive?q=&category=notes`)).text();
    assert.match(html, /Public article 0/);
    assert.doesNotMatch(html, /Enter a word or phrase/);
    assert.ok(publicRequests.some(request => request.path === "/api/v1/articles" && request.query.get("category") === "notes"));
  });
  await t.test("legacy homepage URLs redirect internally with their complete query", async () => {
    const filtered = await fetch(`${origin}/?category=notes&tag=go`, { redirect: "manual" });
    assert.equal(filtered.status, 307);
    assert.equal(filtered.headers.get("location"), "/archive?category=notes&tag=go");
    const searched = await fetch(`${origin}/?q=one&q=two&language=zh`, { redirect: "manual" });
    assert.equal(searched.status, 307);
    assert.equal(searched.headers.get("location"), "/search?q=one&q=two&language=zh");
  });
  await t.test("search and About navigation destinations work", async () => {
    const searched = await fetch(`${origin}/search?q=${encodeURIComponent("中文")}`);
    assert.equal(searched.status, 200);
    assert.match(await searched.text(), /Search results/);
    const searchRequest = publicRequests.find(request => request.path === "/api/v1/articles/search");
    assert.equal(searchRequest.query.get("q"), "中文");
    assert.equal((await fetch(`${origin}/about`)).status, 200);
  });
  await t.test("owner Home, Archive, and search include private publications without sharing them with guests", async () => {
    listMode = "populated";
    for (const path of ["/", "/archive", "/archive?q=private", "/search?q=private"]) {
      const owned = await fetch(`${origin}${path}`, { headers: { Cookie: readerCookie } });
      const ownerHTML = await owned.text();
      assert.equal(owned.status, 200);
      assert.match(ownerHTML, /Private fixture title/);
      assert.match(ownerHTML, /class="private-writing-label"/);
      assert.match(ownerHTML, /noindex/);
      assert.doesNotMatch(ownerHTML, /isaber_dev_session=reader-fixture|private-csrf-fixture/);
      assert.match(owned.headers.get("cache-control") ?? "", /private/);
      assert.match(owned.headers.get("cache-control") ?? "", /no-store/);
      const guestHTML = await (await fetch(`${origin}${path}`)).text();
      assert.doesNotMatch(guestHTML, /Private fixture title|Private fixture summary|Private category fixture|owner-only-body-marker/);
    }
    const protectedRequests = publicRequests.filter(request => request.path.startsWith("/api/v1/admin/reading"));
    assert.ok(protectedRequests.length > 0);
    assert.ok(protectedRequests.every(request => request.cookie === readerCookie));
    assert.ok(publicRequests.filter(request => request.path === "/api/v1/articles" || request.path === "/api/v1/articles/search").every(request => request.cookie === undefined));
  });
  await t.test("private reading is owner-only, uncached, noindex, and hides the public comment form", async () => {
    const owned = await fetch(`${origin}/articles/private-fixture`, { headers: { Cookie: readerCookie } });
    assert.equal(owned.status, 200);
    const html = await owned.text();
    assert.match(html, /owner-only-body-marker/);
    assert.match(html, /Private · Only you/);
    assert.match(html, /noindex/);
    assert.doesNotMatch(html, /id="comments"/);
    assert.doesNotMatch(html, /href="#comments-title"/);
    for (const headers of [{}, { Cookie: "isaber_dev_session=forged" }, { Cookie: `${readerCookie}; isaber_dev_session=forged` }, { Cookie: `isaber_dev_session=forged; ${readerCookie}` }]) {
      const guest = await fetch(`${origin}/articles/private-fixture`, { headers });
      assert.equal(guest.status, 404);
      assert.doesNotMatch(await guest.text(), /Private fixture title|Private fixture summary|owner-only-body-marker/);
    }
  });
  await t.test("expired owner cookies return to public reading without retaining private HTML", async () => {
    readerAuthenticated = false;
    try {
      const html = await (await fetch(origin, { headers: { Cookie: readerCookie } })).text();
      assert.doesNotMatch(html, /Private fixture title|Private fixture summary|owner-only-body-marker/);
      assert.equal((await fetch(`${origin}/articles/private-fixture`, { headers: { Cookie: readerCookie } })).status, 404);
    } finally { readerAuthenticated = true; }
  });
  await t.test("admin page offers sign-in but exposes no owner data", async () => {
    const response = await fetch(`${origin}/admin`);
    assert.equal(response.status, 200);
    assert.match(response.headers.get("cache-control") ?? "", /no-store/);
    const html = await response.text();
    assert.match(html, /Owner sign-in/);
    assert.match(html, /name="password"/);
    assert.match(html, /name="remember_me"/);
    assert.match(html, /Keep me signed in for 30 days/);
    assert.match(html, /method="post"/);
    assert.doesNotMatch(html, /Signed in as/);
  });
  await t.test("authenticated admin HTML is uncached and never reused for anonymous readers", async () => {
    const owner = await fetch(`${origin}/admin`, { headers: { Cookie: "fixture-owner=session" } });
    assert.equal(owner.status, 200);
    assert.match(owner.headers.get("cache-control") ?? "", /no-store/);
    const ownerHTML = await owner.text();
    assert.match(ownerHTML, /private-owner-fixture/);
    assert.match(ownerHTML, /private-csrf-fixture/);
    assert.match(ownerHTML, /aria-label="Workspace"/);
    assert.doesNotMatch(ownerHTML, /name="password"/);
    const anonymous = await fetch(`${origin}/admin`);
    assert.match(anonymous.headers.get("cache-control") ?? "", /no-store/);
    const anonymousHTML = await anonymous.text();
    assert.match(anonymousHTML, /Owner sign-in/);
    assert.doesNotMatch(anonymousHTML, /private-owner-fixture|private-csrf-fixture|aria-label="Workspace"/);
  });
  await t.test("empty search prompts for input", async () => {
    const response = await fetch(`${origin}/?q=`);
    assert.equal(response.status, 200);
    assert.match(await response.text(), /Enter a word or phrase to search public articles/);
  });
  await t.test("repeated search text has an invalid-input state", async () => {
    const response = await fetch(`${origin}/?q=go&q=database`);
    assert.equal(response.status, 200);
    assert.match(await response.text(), /Use a single search query/);
  });
  await t.test("an older API without comments still serves existing articles", async () => {
    const response = await fetch(`${origin}/articles/legacy-article`);
    assert.equal(response.status, 200);
    const html = await response.text();
    assert.match(html, /Existing article body/);
    assert.match(html, /Comments are temporarily unavailable/);
    assert.match(html, /New comments are disabled or unavailable/);
  });
  await t.test("liveness is explicitly uncached", async () => {
    const response = await fetch(`${origin}/healthz`);
    assert.equal(response.status, 200);
    assert.equal(response.headers.get("cache-control"), "no-store");
    assert.deepEqual(await response.json(), { status: "ok" });
  });
  await t.test("missing pages return 404", async () => {
    const response = await fetch(`${origin}/missing-page`);
    assert.equal(response.status, 404);
  });
});
