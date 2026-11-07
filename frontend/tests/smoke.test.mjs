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
  const olderAPI = createServer((request, response) => {
    if (request.url === "/api/v1/articles/legacy-article") {
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
    assert.match(html, /A home for words and ideas\./);
    assert.match(html, /Skip to content/);
  });
  await t.test("admin page offers sign-in but exposes no owner data", async () => {
    const response = await fetch(`${origin}/admin`);
    assert.equal(response.status, 200);
    assert.match(response.headers.get("cache-control") ?? "", /no-store/);
    const html = await response.text();
    assert.match(html, /Owner sign-in/);
    assert.match(html, /name="password"/);
    assert.match(html, /method="post"/);
    assert.doesNotMatch(html, /Signed in as/);
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
