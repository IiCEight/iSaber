# Public-page review — November 12, 2026

Status: Local review completed with two reading-accessibility fixes and the open findings below. This is not final live-content or physical-device acceptance. No remote push or deployment was performed. Biography and contact copy remain deferred at the owner's request.

## Scope and results

The actual React pages were exercised against an isolated, in-memory public API fixture: Home, Archive, English Article, Chinese Article, and About. The fixture contains seven representative entries, including a long unbroken title, mixed English/Chinese passages, a short article without headings, managed-path imagery, code, a wide table, and display math. These are demonstration entries, not the owner's stored articles. They enter the application only through an explicit local `BACKEND_ORIGIN` override.

| Review | Evidence |
| --- | --- |
| Responsive reading | 20 page/viewport combinations at 1440, 768, 390, and 320 CSS pixels; no document-wide horizontal overflow. Code, tables, and equations scroll within their reading areas. Desktop/mobile screenshots inspected. |
| Accessibility scan | Axe WCAG A/AA checks on the five routes at desktop and mobile widths reported no violations after the fixes. This supplements keyboard review; it does not establish complete accessibility conformance. |
| Keyboard reading | Code, tables, and wide equations receive visible focus and respond to arrow keys. Copy code copies the rendered source. Mobile contents links move focus to their target heading. |
| Discovery | Mobile Menu/Escape, Search → Archive input focus, inline search expansion, persistent applied chips, draft-field reset on Escape, and removing one chip while retaining other conditions passed. |
| Motion and navigation | Home lens attraction remains bounded after entrance. Home → Archive → About client navigation restores the correct header behavior. About's pink field fills, slides left, exits, and reverses correctly. Manual and system reduced motion restore linear reading. |
| Enlarged/short layouts | 200% root text at mobile width and a short landscape viewport keep content within the viewport. Home/About use their linear fallbacks. |
| Resilience | Empty collections, no search results, API outages, and article retry recovery were exercised. Unknown article URLs show the generic missing state with JavaScript enabled. |
| JavaScript disabled | Home, Archive, an existing Article, and About render readable content. The missing-article exception is recorded below. |
| Local code checks | TypeScript and targeted Biome lint passed; a local production build passed; all 18 existing frontend tests passed. No remote CI/CD run was started. |

## Fixes

- Highlight.js added a second horizontal scroll area inside the already focusable code region. Article CSS now lets the named outer region own scrolling, removing the nested keyboard obstacle and duplicate padding.
- KaTeX display formulas could overflow on a phone without being reachable by keyboard. The shared Markdown renderer now preserves their markup while adding a named, focusable scroll region. This works in server-rendered HTML without JavaScript; inline formulas retain their existing behavior.

## Open findings and acceptance limits

1. **Missing article without JavaScript:** a missing article returns HTTP 404, but the browser shows a blank body with JavaScript disabled. Reproduced in development and the local production build. The JavaScript-enabled fallback works. A metadata-only change and a blocking-metadata user agent did not resolve it; no speculative routing/configuration workaround was retained. Resolve this before claiming complete no-JavaScript error recovery.
2. **Actual published content not reviewed:** the initial review used the former IP address, `https://101.42.13.215`, and encountered certificate hostname verification failures. That observation is historical: domain setup has since merged in PR #31. The supported canonical address is now [https://isaber.top](https://isaber.top), with `www` redirected to it; see the [domain setup record](../operations/deployment.md#domain-setup). Use the canonical domain for owner sign-in and live-content acceptance. The old IP failure is not evidence of a problem with the new domain; the local fixture checks above still do not establish real-content acceptance.
3. The fixture is not the Go backend. It does not verify Typora uploads, real image authorization, publication, moderation, or comment submission. Existing automated backend/CI evidence remains separate. Physical-device touch/performance and Safari/Firefox review also remain open.
4. Keep the current approved visual direction. Final typography choices and the owner's biography/contact content need owner review; the latter is explicitly deferred.

## Repeat the local visual review

Use the repository's pinned Node version from `frontend/.nvmrc`. Run these commands from the repository root, in separate terminals:

In the current WSL setup, make the installed Linux toolchain available in each terminal first:

```bash
export PATH="$HOME/.local/share/isaber/toolchains/node-v24.22.2-linux-x64/bin:$PATH"
```

```bash
# Terminal 1: isolated content fixture; no database or persistent writes.
node docs/design/review-content.mjs
```

```bash
# Terminal 2: actual frontend pointed explicitly at the fixture.
BACKEND_ORIGIN=http://127.0.0.1:18080 NEXT_TELEMETRY_DISABLED=1 \
  npm --prefix frontend run dev -- --port 3001
```

Open:

- Home: <http://127.0.0.1:3001/>
- Archive: <http://127.0.0.1:3001/archive>
- English reading study: <http://127.0.0.1:3001/articles/reading-study>
- Chinese reading study: <http://127.0.0.1:3001/articles/chinese-study>
- Short article: <http://127.0.0.1:3001/articles/field-note-3>
- About: <http://127.0.0.1:3001/about>

If the computer requests reduced motion, a static preview is expected. Use the operating system/browser preference when comparing motion; About also has a Reduce motion control.

For local empty/outage checks only, change the fixture's in-memory state:

```bash
curl -X POST 'http://127.0.0.1:18080/__review/mode?value=empty'
curl -X POST 'http://127.0.0.1:18080/__review/mode?value=outage'
curl -X POST 'http://127.0.0.1:18080/__review/mode?value=normal'
```

Refresh the page after a state change. For recovery, switch to normal and use the Article retry button. Stop both servers with Ctrl+C when done. Port 3001 is for fixture review; normal frontend development without this override continues to use the real Go API. Never run this fixture as a production backend.
