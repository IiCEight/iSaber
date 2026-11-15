# Owner reading view — local review

Status: Implemented locally on `fix/owner-private-reading` after the owner approved a shared reading experience. No remote push, deployment, or database migration is part of this iteration.

## Reading flow

Guests see public publications on Home, Archive, Search, and article pages. Open `/admin` directly to sign in, then return to the blog: the same URLs now include published private writing, identified by a small outlined Private label. Home keeps its four recent publications; Archive shows the complete visible collection with existing filters, date sorting, and numbered pagination. Private-only published categories/tags become available only in the owner view. Drafts remain in the administrative preview workflow.

A private article opens at its normal `/articles/{slug}` address with the existing full reading layout, contents, Markdown, code, math, dates, and protected images. Its label reads “Private · Only you”. Public comments and their contents link are omitted. The admin article settings provide Read private article ↗ for private publications. Website navigation shows Archive, About, and Search for both guests and the signed-in owner. Login and workspace access use the direct `/admin` URL.

Signing in refreshes already-open reading tabs. Signing out reloads the administrative document and other open owner reading tabs, removing their private collection/content. Focus, route navigation, restored browser pages, and session-expiry checks revalidate the session with the backend. Browser Back after logout cannot regain private content from an old owner render. Expiry decisions remain authoritative on the server even if the browser clock is ahead.

## Access boundary

The public API stays cookie-independent and public-only, including when the caller supplies a valid owner cookie. New protected endpoints under `/api/v1/admin/reading/` reuse the existing browse/search implementation with a server-attached owner scope. Only the existing authentication middleware can establish that scope. The endpoints expose published public/private article summaries, current publication detail, and labels used by those publications. Cursor scopes are distinct from public and administrative-management cursors.

Next.js validates supported session cookies before selecting the protected reading endpoints. Raw duplicate session cookies are preserved for backend rejection; unrelated cookies are not forwarded. Public collection/detail/label calls receive no cookie. A rejected owner request falls back to the public endpoint. The session cookie and CSRF token are never serialized into reading-page HTML.

Protected reading responses use private/no-store and robots exclusion. Owner-rendered HTML remains dynamic and private/no-store, with noindex metadata. Missing, private, and unpublished article pages are indistinguishable to guests. Images continue to require independent authorized delivery; logout/expiry removes private-image access too. Browser auth-change messages contain only a state name, never credentials or tokens.

## Verification

The full backend race/integration suite and Go vet pass. The new regression checks public/owner totals and all six date orders, private body search, draft exclusion, published labels, cursor isolation, direct access, image authorization, and revocation. All 29 frontend tests pass, including consecutive owner/guest SSR requests, private content/metadata exclusion, expired/forged/duplicate cookies, no-store/noindex behavior, and public-only cookie-free API calls. Type checks, Biome, the production build, and the 36-operation OpenAPI contract pass.

Chromium exercised the complete local login-to-reading flow, existing-tab updates, normal private reading and images, filters/search, admin Read action, cross-tab sign-out, browser Back after logout, and expiry on focus. Home, Archive, and the article reader were checked at 1440, 1024, 768, 390, and 320 CSS pixels. Preview data uses public summaries and synthetic bodies; the private sample and test image exist only in the disposable local database.

Review screenshots: [desktop Home](previews/owner-reading-home-desktop.png), [mobile Home](previews/owner-reading-home-mobile.png), [desktop Archive](previews/owner-reading-archive-desktop.png), [mobile Archive](previews/owner-reading-archive-mobile.png), [desktop reader](previews/owner-reading-article-desktop.png), and [mobile reader](previews/owner-reading-article-mobile.png). Local review runs at <http://127.0.0.1:3001/> with the existing synthetic local preview account.
