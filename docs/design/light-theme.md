# Light and dark reading surfaces

Status: Implemented locally, with scope narrowed to the owner's annotated screenshots. The selected light reading background is `#FFE2ED`. The owner authorized committing and pushing the reviewed changes on November 16, 2026. Application deployment follows the normal PR and main-branch pipeline.

## Design boundaries

- **Home:** only recent-article/error panels, the “View archive” link, and the lower explore/footer panels use the selected reading palette. The black navigation, hero title fields, introduction card, primary hero links, vivid canvas, circles, and motion retain their original appearance.
- **Archive:** the collection area below the masthead—including article rows, inline search, sorting, pagination, and empty/error states—uses the reading palette. The masthead remains blue with white text, and the footer retains its original blue.
- **Article:** the reading column, contents rail, code, math, tables, comments, and reading footer use the selected palette. The warm opening and black navigation retain their original colors.
- **About and admin:** retain their original palettes. The wider palette changes from the earlier iteration were removed.

A light/dark switch remains in public navigation. On mobile it has a sun/moon icon, an accessible action label, and a 44px target. Changing themes affects the reading surfaces rather than recoloring the artwork or navigation.

## Preference behavior

A new visitor receives light mode even if the operating system prefers dark mode. The server validates the presentation-only `isaber_theme` cookie and emits `data-theme` before page content. Missing/invalid preferences resolve to light. The client remembers a selection for one year, using a host-only, path `/`, SameSite=Lax cookie, with Secure on HTTPS. No account, database, or theme API is needed.

Switching retains page content, filters, form drafts, scroll position, and motion state. Returning tabs synchronize their saved preference on focus/page restoration. With JavaScript disabled, the server still renders the saved palette and disables the enhancement button. If cookies are blocked, the current page can switch, but persistence across visits is unavailable.

## Color controls

`frontend/app/theme.css` defines the reading colors in `:root` and `:root[data-theme="dark"]`. `--site-bg` is the reading background. `--site-header`/`--site-header-ink` remain black/white in both palettes; `--site-masthead` remains the original blue gradient. Decorative artwork colors remain in each page's existing CSS.

## Preview with actual published articles

The local Go API is not required for this visual review. `public-preview.py` serves a saved anonymous public API response for **The Art of Readable Code**. Home and Archive show that single article as a selected preview collection. The snapshot contains the real published Markdown and metadata, with no database import or online modification. The helper makes no network requests, ignores account cookies, returns an anonymous session, and rejects all writes. Comments are reported unavailable rather than inventing a comment history. This helper is for visual review, not validation of the Go API's search or sorting behavior.

Use the pinned Linux Node toolchain. Save a public article JSON response, then run the two servers in separate terminals:

```bash
curl -fsS --retry 3 'https://isaber.top/api/v1/articles/the-art-of-readable-code' \
  -o /tmp/isaber-public-article.json
```

```bash
# Terminal 1
python3 docs/design/public-preview.py --article-json /tmp/isaber-public-article.json
```

```bash
# Terminal 2; stop any previous frontend dev server from this checkout first.
export PATH="$HOME/.local/share/isaber/toolchains/node-v24.22.2-linux-x64/bin:$PATH"
BACKEND_ORIGIN=http://127.0.0.1:18080 NEXT_TELEMETRY_DISABLED=1 \
  npm --prefix frontend run dev -- --port 3000
```

Review:

- <http://127.0.0.1:3000/>
- <http://127.0.0.1:3000/archive>
- <http://127.0.0.1:3000/articles/the-art-of-readable-code>

If a previous selection is dark, use the **Light** button to see pink reading surfaces. Once the snapshot is downloaded, the preview works offline. Stop both servers with Ctrl+C when finished. Normal development can use the actual Go API by starting Next without the preview `BACKEND_ORIGIN` override.

## Verification

TypeScript, Biome, and whitespace checks pass. Sixteen light/dark desktop/mobile browser views passed. Checks compare original Home/nav colors, pink reading panels, the blue/white Archive masthead, and the original About card. Actual public article content is displayed locally. Light/dark preference persistence, page bounds, accessibility contrast, anonymous preview session behavior, and rejection of write requests are checked locally. No remote pipeline or new production build is run for this correction.

Earlier local checks covered preserved animation positions, search drafts, keyboard toggling, no-JavaScript reading, and blocked-cookie fallback; those interaction mechanisms are unchanged by this visual scope correction. Physical-device/cross-browser release acceptance remains separate.

Saved captures:

- [Home reading panels](previews/theme-home-panels-light-desktop.png)
- [Archive, desktop](previews/theme-archive-light-desktop.png)
- [Archive, mobile](previews/theme-archive-light-mobile.png)
- [Article reading](previews/theme-article-light-reading.png)
