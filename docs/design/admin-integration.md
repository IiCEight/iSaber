# Admin dashboard integration

Status: The owner approved the conventional dashboard prototype and confirmed that its simplified Markdown preview did not need further work. The design is now integrated locally into the actual `/admin` application. No remote push or deployment is part of this iteration.

## Result

The authenticated workspace uses the approved dark sidebar, light canvas, white panels, compact article table, blue actions, and textual status badges. Articles, Import article, Categories & tags, and Comments are separate workspace views. The article editor has Metadata, Content preview, and Published versions views with a separate publication/settings column. Phone layouts use horizontal navigation, stacked fields and panels, and keyboard-focusable scrolling for wide tables.

The sign-in screen uses the real username/password flow and shows backend availability, invalid-credential, rate-limit, and expired-session feedback. The dashboard displays the authenticated username. Sign-out uses the existing session endpoint. The public navigation is replaced by the workspace header only on admin routes; the public pages retain their existing styles and behavior.

All displayed data comes from the existing Go API. The owner article-summary endpoint does not include category or language, so table rows show their real URL and unpublished-draft indicator instead of introducing extra requests or fabricated metadata. Article/comment counts describe loaded rows, not an invented collection total. Cursor-based continuation is preserved. There are no prototype controls, sample entries, fake statistics, or authentication bypasses in the React application.

## Writing and publication

Write in Typora and import Markdown or ZIP. Native file selection and optional drag-and-drop use the same import handler, size/type guidance, upload fingerprint/idempotency key, bounded job polling, and Resume import path. A new import starts as a private draft. A revision import leaves the published version intact.

The metadata form preserves title, pre-publication URL, summary, language, writing date, category, and tag membership. Published URLs remain locked, including after unpublication. Published articles without a draft show read-only metadata; import a draft revision or restore a published version to edit. Unavailable labels disable metadata writes and offer refresh rather than silently clearing saved associations.

The content preview uses the actual shared Markdown renderer, including GFM headings/lists/tables, syntax highlighting, managed images, and KaTeX. Raw HTML is skipped. The same renderer displays current published and historical versions. Markdown H1 headings are leveled below the workspace page title. Code, tables, and wide formulas have keyboard-accessible local scroll regions.

Unsaved metadata prompts before workspace navigation, article reload/selection, or sign-out; leaving/reloading the document also uses the browser's unsaved-change warning. Switching editor views retains the form. Metadata edits invalidate the preview. Publication requires the current saved draft, a matching reviewed revision, resolved blocking issues, and explicit confirmation. The preview response's article version is checked so an externally changed article must be reloaded before reviewing/publishing. Visibility and new-comment settings remain separate immediate operations.

Restoration creates a draft and requires a fresh preview before publication. Unpublication retains the draft and history. Native dialog confirmations default to Cancel, support Escape and keyboard focus, and resolve canceled if their provider unmounts. Confirmation text describes the actual operation and its effect.

## Existing protections

No API routes, database schema, or backend permissions changed. Session/CSRF handling, HttpOnly cookies, owner authorization, expected versions, retry keys, publication validation, and private image delivery remain in the existing request path. CSRF tokens remain in memory; no new browser storage is introduced. Admin responses remain dynamic and uncached, with robots indexing disabled.

Label creation/rename use the existing identifiers and version checks; URL names remain fixed when renamed. Deletion is not introduced. Moderation retains Pending/Approved/Rejected/Removed states, article scoping, pagination, explicit confirmation, and retained removed comments. The queue loads on entry/filter change; sequence checks ignore late responses from a previous view/filter. Queue errors are distinct from a successfully empty result.

Styles are restricted to the admin container so Next.js retaining a route stylesheet after navigation does not restyle the public pages. The implementation adds no dependency or component library.

## Local verification

- TypeScript and Biome pass. A local production build passes.
- Chromium reviewed eight authenticated/sign-in/editor screens at 1440, 768, 390, and 320 CSS pixels: 32 screen/viewport combinations without document-wide overflow or uncaught browser errors. Desktop/mobile screenshots were inspected.
- Desktop/mobile WCAG A/AA scans passed for the normal screens. Preview scans identified an extra nested code scroll area, which was corrected; focused preview scans then passed at both widths. Code, table, and formula regions were exercised with keyboard input and visible focus.
- Browser checks exercised actual frontend requests through Next.js rewrites against a temporary authenticated API fixture: sign-in errors/password clearing, pagination/filtering, metadata save, unsaved-change cancellation, full English/Chinese Markdown preview, historical reading, restore, publication cancellation/confirmation, multipart revision import, visibility/comments, label rename, moderation, version conflicts, session expiry, and sign-out. CSRF headers, expected versions, and idempotency keys were checked.
- Failure checks verified article-list retry, label retry without dropping tag memberships, and moderation retry after API outages. Leaving admin restored the correct public header/layout.
- All 19 frontend tests pass, including a production smoke regression checking that authenticated admin HTML is uncached and never reused for a following anonymous request. The temporary fixture is not a substitute for live Go/database acceptance; no real account, stored article, or cloud server was modified.

Physical-device and full real-Go publishing acceptance remain separate checks. The earlier public-page review's no-JavaScript missing-article finding remains open. Its IP certificate observation is historical: PR #31 established `https://isaber.top` as the canonical address, with `www` redirected to it. Live owner acceptance should use the new domain and the existing credentials; see the [domain setup record](../operations/deployment.md#domain-setup).

## Local review

Use the normal [local development setup](../../README.md#local-development), with the Go API at `http://127.0.0.1:8080` and the frontend at `http://127.0.0.1:3000`. Open <http://127.0.0.1:3000/admin> and sign in with the local owner account. If the API is stopped, the sign-in screen displays an availability message; no mock account is supplied by the application. The static [prototype](http://127.0.0.1:8765/admin-prototype.html) remains a design reference.

At this handoff the temporary QA backend was stopped, the normal frontend was started on port 3000, and the local Go API on port 8080 was not reachable. Start the normal local API to review the authenticated workspace with your own account/data.
