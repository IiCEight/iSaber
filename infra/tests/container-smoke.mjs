// Run inside the frontend container, with the real Go API and migrated database.
import assert from "node:assert/strict";
assert.notEqual(process.getuid(), 0, "frontend must run without root");
async function check(path, status) {
  const response = await fetch(`http://127.0.0.1:3000${path}`, { signal: AbortSignal.timeout(10000) });
  assert.equal(response.status, status, path);
  assert.match(response.headers.get("cache-control") ?? "", /no-store/, path);
  return response;
}
assert.equal((await fetch("http://api:8080/readyz")).status, 200);
assert.deepEqual(await (await check("/healthz", 200)).json(), { status: "ok" });
const home = await (await check("/", 200)).text();
assert.match(home, /id="home-title"/);
assert.match(home, /The first chapter is on its way/);
assert.match(await (await check("/archive", 200)).text(), /<h1 id="archive-title">Archive\.<\/h1>/);
assert.match(await (await check("/search", 200)).text(), /Enter a word or phrase/);
await check("/about", 200);
const staticPaths = [...home.matchAll(/(?:src|href)="([^"?]+\.(?:js|css))(?:\?[^"]*)?"/g)].map(match => match[1]);
assert.ok(staticPaths.length > 0, "SSR must reference static assets");
for (const path of staticPaths) {
  assert.equal((await fetch(new URL(path, "http://127.0.0.1:3000"))).status, 200, path);
}
assert.match(await (await check("/admin", 200)).text(), /Owner sign-in/);
assert.deepEqual((await (await check("/api/v1/articles", 200)).json()).items, []);
await check("/api/v1/admin/articles", 401);
const rejected = await fetch("http://127.0.0.1:3000/api/v1/articles/11111111-1111-4111-8111-111111111111/comments", {
  method: "POST", headers: { Origin: "https://untrusted.example", "Content-Type": "application/json" }, body: "{}",
});
assert.equal(rejected.status, 403, "API rewrite must preserve origin enforcement");
console.log("Container readiness, SSR, static assets, API rewrite, and authorization passed.");
