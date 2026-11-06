import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { once } from "node:events";
import { createRequire } from "node:module";
import { setTimeout as delay } from "node:timers/promises";
import test from "node:test";
import { fileURLToPath } from "node:url";

const require = createRequire(import.meta.url);
const root = fileURLToPath(new URL("../", import.meta.url));
const origin = "http://127.0.0.1:4317";

test("production frontend serves its shell without a running backend", async (t) => {
  const server = spawn(process.execPath, [require.resolve("next/dist/bin/next"), "start", "--hostname", "127.0.0.1", "--port", "4317"], {
    cwd: root,
    env: { ...process.env, NODE_ENV: "production", NEXT_TELEMETRY_DISABLED: "1" },
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
  await t.test("admin shell exposes no management form or data", async () => {
    const response = await fetch(`${origin}/admin`);
    assert.equal(response.status, 200);
    assert.match(response.headers.get("cache-control") ?? "", /no-store/);
    const html = await response.text();
    assert.match(html, /Sign-in is not available yet\./);
    assert.doesNotMatch(html, /<form[\s>]/);
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
