# API design

Status: Approved by the owner as the API design baseline. The machine-readable contract is [OpenAPI](../../contracts/openapi.yaml); this document explains conventions and end-to-end workflows. The [data model](data-model.md) defines persistence invariants.

Implementation status: owner authentication, Markdown/ZIP imports and authorized images, published revision history, labels/search and guest comments/moderation are merged and deployed. The [publishing guide](../operations/publishing.md) lists current routes and limits. This document and OpenAPI define the approved API behavior; [ADR 0019](decisions/0019-simple-delivery.md) records the revised deployment scope.

## 1. Boundaries and naming

Use REST-style JSON endpoints under `/api/v1`. Public routes are under `/articles`, `/categories`, and `/tags`. Owner routes are under `/admin`. Session routes are under `/auth`. These are API paths; the React dashboard remains `/admin`.

Public article routes return only current public published data, even when the caller happens to have an owner session. Private and historical content is available through protected admin endpoints. The asset route is the explicit exception: it permits an authenticated owner to read retained assets from that article.

Unknown and inaccessible public resources return the same `404` problem. API responses never include private storage keys, password hashes, session tokens, or pending comment text for visitors.

## 2. Authentication

Use owner username/password login and opaque server-side sessions. The implementation uses Argon2id, an eight-hour absolute session lifetime, a 30-minute idle limit, and bounded per-process login attempts. Setup and atomic credential recovery are operational commands, not public APIs. See [ADR 0005](decisions/0005-owner-authentication.md) for parameters and the [owner guide](../operations/owner-account.md) for setup and deployment requirements.

- Login creates a fresh `__Host-isaber_session` cookie with `Secure`, `HttpOnly`, `SameSite=Strict`, and `Path=/`, with no `Domain` attribute. The cookie value is never returned in JSON.
- Session responses return a CSRF token. `GET /auth/session` restores owner information and that token after a page reload; unauthenticated requests return `401`.
- Derive the CSRF token reproducibly on the server from the session cookie using a separate server secret, retaining only verification material in the database. This supports reloads without storing recoverable session bearer tokens or invalidating other tabs' CSRF tokens.
- Every unsafe owner operation except login requires the session cookie and `X-CSRF-Token`. Verify the token against the session on the backend.
- Every browser mutation, including login, multipart import, and guest comments, must have an `Origin` matching the configured site origin. Reject missing or mismatched origins in this browser-only API. JSON operations accept only their declared media type. Do not enable credentialed cross-origin access.
- Sign-out revokes the session and clears its cookie. Successful recovery revokes all owner sessions.
- Same-origin routing keeps the public site, dashboard, and API on one origin. Local development uses HTTPS or a clearly isolated development cookie configuration that cannot be used in production.

The owner approved these session and CSRF mechanisms; their implementation is merged in PR #5. Development uses a separate loopback-only HTTP cookie, while production requires HTTPS and a stable injected CSRF key. No reader accounts are introduced.

## 3. Responses and errors

Successful single-resource responses are direct objects. Lists return `items` and `next_cursor`. IDs are UUIDs; event times are UTC RFC 3339 values; writing dates use `YYYY-MM-DD`. A missing publication timestamp is JSON `null`, not an invented upload date.

Public article browse/search endpoints also support numbered pagination: supply `page` (positive integer) and optionally `limit` (page size, default 20). These responses add `page`, `page_size`, `total`, and `total_pages`, with `next_cursor: null`. Totals include only matching current public published articles. Counts and results use one repeatable-read snapshot. A page beyond the available collection resolves to its last page; an empty collection uses page 1 of 1. Numbered browse and search default to `sort=written` (newest writing date first). Other choices are `written_oldest`, `updated`, `updated_oldest`, `newest`, and `oldest`; the latter two use first publication time. Search additionally accepts `relevance`. Last updated uses the live article timestamp; writing date comes from the published revision. The chosen date orders all matches, ID breaks ties in the same direction, and missing dates sort last. Sorting requires `page`; do not combine `page` and `cursor`. Omitting `page` preserves the existing cursor response and semantics. No schema migration is required.

Archive uses shareable `/archive?page=2` links with retained search/category/tag/language conditions and explicitly requests 10 articles per page. Filter changes reset to page 1. Older Archive cursor URLs redirect to the first page with their conditions preserved; API cursor clients and the separate `/search` route retain their existing behavior.

Errors use `application/problem+json` with `type`, `title`, `status`, `code`, optional `detail`, and `request_id`. Validation errors can include a field/path-based `errors` array. Never include raw credentials, private text, or stack traces.

| Status | Typical use |
| --- | --- |
| 400 | Invalid query, malformed request, or invalid cursor |
| 401 | Missing/expired owner session or failed login |
| 403 | Invalid origin or CSRF token |
| 404 | Missing/inaccessible resource |
| 409 | Version conflict, duplicate slug, invalid lifecycle transition, idempotency conflict, or request in progress |
| 413 | Upload/request too large |
| 415 | Unsupported request media type |
| 422 | Invalid metadata or blocked publication |
| 429 | Rate limit; include `Retry-After` |
| 500/503 | Unexpected error or temporarily unavailable dependency |

Draft issues are distinct from HTTP errors. An import can finish successfully with a saved draft whose `publishable` is false and whose issues identify missing metadata or images. A structurally invalid archive fails the import job. Polling a failed job still returns `200` with its failure status and safe issue details.

## 4. Lists and search

Use opaque cursors and `limit` (default 20, maximum 100). A cursor encodes the ordering position and relevant query/filter context; reject reuse with different filters. Return `next_cursor: null` at the end. Lists are not a cross-request snapshot; concurrent publication can change results.

- Public articles: `published_at DESC, id DESC`.
- Search: all whitespace-separated terms must match in title, summary/tag names, or parsed Markdown body. Rank by the highest field level needed to satisfy all terms, then first publication time and ID descending. `q` is required, nonblank, and at most 200 Unicode characters. ASCII identifiers match whole words ignoring case; Chinese and punctuation terms match literal substrings. Empty queries return 400, while no matches return an empty ArticlePage. See [ADR 0009](decisions/0009-public-search.md) for matching, cursor, and execution details.
- Owner articles: `modified_at DESC, id DESC`; `q` searches draft title or latest retained title, not private content through public endpoints.
- Revision history: only previously published snapshots, newest first, with stable ID tie-breaker. Preview accesses the current draft separately.
- Comments: public by creation time ascending; admin by creation time descending.
- Labels: case-sensitive Unicode code-point name order then ID (`COLLATE "C"`), with cursors bound to label kind and public/admin audience.

Public filters are category slug, tag slug, and language (`en`, `zh`). Admin article filters add publication status and visibility. The backend enforces allowed filter combinations and parameter length limits. All public filtering operates after visibility authorization; counts and label lists cannot leak private material.

## 5. Owner publishing workflow

1. `POST /auth/login`; retain the CSRF token and let the browser manage the cookie.
2. `POST /admin/imports` with a multipart file, CSRF header, and idempotency key. For an update, include `article_id` and its `expected_version`; omit both for a new article.
3. Poll `GET /admin/imports/{import_id}` until `succeeded` or `failed`. A `202` acceptance does not imply validation or publication succeeded.
4. On success, fetch `GET /admin/articles/{article_id}`. Inspect `draft`, `issues`, and `publishable`. Missing metadata/images may require correction.
5. `PATCH /admin/articles/{article_id}/draft` to save metadata and a pre-publication slug. This creates a snapshot and returns the updated article version. Re-upload fixes article-body or local-image issues.
6. `GET /admin/articles/{article_id}/preview` for the draft's normalized Markdown and safe asset references. The React renderer uses the same rendering specification as public reading. Preview returns `404` if there is no draft.
7. `PATCH /admin/articles/{article_id}` to explicitly select public/private visibility or comment settings. This applies immediately; on an existing article a privacy change must not wait for a new draft publication.
8. `POST /admin/articles/{article_id}/publish` with the current `expected_version` and an idempotency key. Validation failures return `422`; stale versions return `409`.
9. Fetch the public article by slug if public, or the admin detail if private. No publication operation returns a bypass link for private content.

History is listed under `/revisions`; individual snapshots are owner-readable. `POST .../restore` copies a previously published revision into the draft. `POST .../unpublish` withdraws live content immediately. Restoring replaces an existing draft pointer with a new snapshot, copying its body/metadata/assets and recording the historical source. It does not change article publication/update dates, access/comment controls, or live pointers. Draft-only snapshots are excluded from history. All of these mutation operations check versions. If a response is lost, retry with the same idempotency key before fetching current state; do not silently resubmit a conflicting edit.

### Front matter mapping

Recognize `title`, `summary`, `slug`, `language` (`en` or `zh`), `written_at` (`YYYY-MM-DD`), optional `updated_at` (RFC 3339 timestamp with timezone), `category` (display name), and `tags` (list of display names). The draft metadata PATCH also accepts nullable `updated_at`; publication applies that choice, with null using publication time. Each uploaded file starts with automatic dating unless it supplies its own timestamp. Resolve category/tag names against existing labels. Unknown or ambiguous labels are blocking draft issues; the owner can create or select labels through the dashboard. Do not silently create labels from imported text. Unknown front matter keys produce warnings and are not executed or treated as application settings.

On a new article, a valid supplied slug is used if available; otherwise suggest a unique slug for owner confirmation. A slug conflict is a blocking issue, not permission to overwrite another article. On replacement, the article identity and existing slug win over imported slug values. Global visibility and comment controls remain unchanged; imported `visibility` or `comments_enabled` values produce a warning directing the owner to explicit dashboard controls. System publication/update timestamps cannot be supplied by front matter. These rules prevent a replacement file from silently publishing content or changing access.

The normalized Markdown excludes parsed front matter and uses managed image routes. Preserve unrecognized non-executable content as text or an explicit preview warning under the rendering specification. The original staged upload is not a publicly downloadable asset.

## 6. Taxonomy and comments

Owner category/tag endpoints create labels and rename their display names using versions. Slugs remain stable; deletion and merging are deferred. Unknown or conflicting references block draft publication with actionable issues.

Guest comment submission requires a UUID idempotency key and returns only `{status: pending}`. It never issues an account or returns the submitted text as a public resource. Submission is allowed only for a currently public published article with comments enabled. Disabled comments return `409 comments_disabled`; nonpublic articles return `404`.

Comment limits are five accepted submissions per article and twenty globally per sliding minute, coordinated in PostgreSQL; retries do not consume quota. Two per-process admission slots bound concurrency. Validate exact Origin, bounded text, and a UUID idempotency key; recheck article eligibility before both insert and replay. See [ADR 0010](decisions/0010-guest-comments.md) for persistence and UI retry semantics.

Owner comment lists can filter by article and moderation status. A versioned moderation update selects `approved`, `rejected`, or `removed`. All public reads recheck article visibility, including comment-list pagination.

## 7. Delivery, retries, and compatibility

Article, search, preview, management, session, and asset responses initially use `Cache-Control: no-store`; public pages must also avoid static or shared caches that bypass backend authorization. Media is streamed through the authorized asset route from private storage.

Idempotency semantics and publication transactions follow the data model. Scope and retain keys for 24 hours; reject changed payloads and in-progress duplicates explicitly. Repeat attempts after expiry require state inspection; the server does not promise replay beyond retention.

Add optional response fields compatibly. Changes to required requests, response meaning, public access, or enum values require compatibility review; do not assume every enum extension is safe for generated clients. Retain `/api/v1` compatibility across independently deployed frontend/backend versions. The contract is approved; select code-generation tooling before generating clients.

## 8. Requirement coverage

| PRD capability | Contract area |
| --- | --- |
| AUTH-01 | `/auth/*`; setup/recovery commands documented outside the HTTP API |
| ADMIN-01 | `/admin/articles`, `/admin/imports`, `/admin/categories`, `/admin/tags`, `/admin/comments` |
| PUB-01 | Import jobs and article asset delivery |
| PUB-02 / PUB-03 | Draft, preview, publish, unpublish, revision history, restore |
| META-01 | Revision metadata, article controls, category/tag management |
| READ-01 | Public article Markdown and assets; rendering is a frontend responsibility |
| FIND-01 | Public lists, filters, labels, and search |
| PRIV-01 | Protected admin routes, filtered public routes, authorized assets, no-store policy |
| COMM-01 | Public comment submission/list and protected moderation |
| DESIGN-01 | Frontend design and acceptance checks; no visual design is implied by API approval |
| PORT-01 | Frontend configuration; no portfolio CRUD API required |

## References

- [OpenAPI 3.1.1](https://spec.openapis.org/oas/v3.1.1.html): contract structure, schemas, multipart requests, and security schemes.
- [RFC 9457](https://www.rfc-editor.org/info/rfc9457/): problem details for HTTP APIs.

These standards define contract and error formats; lifecycle, version fields, and idempotency policies above are approved project design decisions.
