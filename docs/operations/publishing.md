# Markdown publishing

Status: Publishing, images, and revision history are merged through PR #8 with successful GitHub checks. Category/tag management and public label browsing are implemented in `feat/article-labels`, pending review. Cloud deployment and the final artistic design remain separate work.

## Try it locally

Start PostgreSQL and follow the root README to configure Go and Next.js. Apply pending migrations before starting the API (the image slice uses the existing version 4 schema):

```bash
cd backend
set -a
. ./.env.example
set +a
go run ./cmd/migrate up
go run ./cmd/api
```

If you already provisioned the owner, keep that account. Run `owner setup` only for a fresh database. Next.js runs in a second terminal with `cd frontend && npm run dev`.

Save this UTF-8 file as `first-article.md` in Typora:

````markdown
---
title: My first article
language: en
written_at: 2026-11-07
summary: Testing my publishing workflow.
---

Hello, world! 中文也可以。

```go
fmt.Println("Hello, blog")
```

Inline math: $E = mc^2$.

$$
\int_0^1 x^2\,dx = \frac{1}{3}
$$
````

1. Open `http://127.0.0.1:3000/admin` and sign in.
2. Upload the file into **New private draft**. Wait for import completion.
3. Review metadata, URL name, and issues. If no writing date was supplied, enter it and save metadata. Preview the draft.
4. Choose **Make public** if you want visitors to read it. This changes access immediately but does not publish a draft.
5. Choose **Publish draft**. Read it from the public homepage or `/articles/{slug}`. A private publication stays available through **Read current version** in the dashboard.
6. Change the file in Typora and upload into **Selected article, as a draft revision**. Visitors continue reading the previous version until you preview and publish the new draft.
7. Try **Make private** and **Unpublish**. A fresh public request must return 404 and the homepage must omit it. Unpublishing retains a draft and stored history.

All new articles start private. Front matter cannot publish an article or change access. The URL locks after first publication. Writing dates are calendar dates; first publication time stays fixed, while publication/control changes update the system update time. Owner and public list endpoints support cursor pagination; the dashboard adds title, status, and visibility filters.

## Include Typora images

Keep the article and its image folder together. For example:

```text
article/
  first-article.md
  images/
    diagram.png
```

Use Markdown image syntax in the article:

```markdown
![Diagram](images/diagram.png)

![Reference image][diagram]

[diagram]: images/diagram.png
```

ZIP the Markdown and the images together, preserving relative folders, then choose the ZIP in **Import article**. A common parent folder inside the ZIP is supported, as are English/Chinese filenames and URL-encoded spaces in image references. Paths resolve relative to the Markdown file; `../images/photo.png` works from a nested article folder when the result stays inside the ZIP. A path pointing outside the archive is never read from your computer or the server.

Preview before publishing. Included images are validated and references become `/api/v1/articles/{article_id}/assets/{asset_id}` URLs automatically. Repeated references share one asset within that import. Markdown examples inside code fences are preserved. A Markdown-only upload with unavailable local images remains a draft and asks you to re-upload a ZIP. Use ordinary Markdown images: raw HTML `<img>` tags from resizing/export tools are skipped by the renderer.

Images belonging to a new draft require owner sign-in, even if an older revision is public. Publishing atomically changes which images visitors can access. Images from replaced revisions remain owner-readable through their retained memberships; guessing their URLs does not grant visitors access. Making private or unpublishing hides image URLs on subsequent visitor requests too. No public object-storage links or redirects are issued.

## Read and restore published versions

1. Select an article in `/admin` and choose **Load published versions**. The list includes current and previous snapshots that were explicitly published, including private publications and versions retained after unpublishing. Draft-only uploads and metadata edits do not appear in this list.
2. Choose a version to read it with the same Markdown/code/math/image renderer. Version dates belong to that snapshot. Use **More versions** for older pages, or **Refresh versions** to reload the list.
3. Choose **Restore selected version as draft**. If a draft exists, the confirmation explains that its pointer will be replaced. The published version remains live. There is no draft-only history browsing in this release.
4. Review the restored title, summary, language, writing date, category/tags, and images. Preview the new draft, then explicitly publish it if you want to replace the live version.

Restoring creates a new snapshot and increments the article version. It copies historical Markdown, metadata, and image memberships; the original snapshot stays immutable. Its `restored_from_revision_id` identifies the selected source, including after metadata corrections to the restored draft. The restored draft starts without a publication timestamp. Publishing it creates a new published history entry.

A restoration preserves the stable URL, current visibility/comment setting, first article publication date, current update date, live content, and comments. It changes only the draft pointer, version, and administrative modified time. Publication later changes the live content and update date. Restoring an unpublished/private article does not make it public. Restoring does not recreate missing image bytes; publication still validates image availability.

History list/detail/restore endpoints are owner-only. A revision must belong to the selected article and have publication history; draft-only, unknown, and cross-article revision IDs return 404. Cursors order by first publication time then ID descending, and cannot be reused for a different article. Unknown articles return 404; an existing article with no published versions returns an empty page.

Restore requests require `expected_version`, CSRF/origin validation, and a UUID idempotency key. Stale versions return 409. Retrying the same restore key/body returns its original result without replacing a later draft again. Reusing that key for a different source or expected version returns 409. The dashboard fetches the latest article after a confirmed restoration; always reload after a conflict before making another edit.

## Organize with categories and tags

1. Open **Categories and tags** in dashboard navigation. Create a label with an English or Chinese name and a stable URL name such as `technology` or `chinese-notes`.
2. Select an article with a draft. Choose zero or one category and check the desired tags in **Draft metadata**, then save. **None** and unchecked tags clear those associations. If the article has no draft, restore a published version or upload a replacement first.
3. Preview and explicitly publish to change the public article's labels. Private articles and new drafts do not expose labels publicly.
4. Read the public article's category/tag links, or use the homepage category, tag, and language filters. Pagination preserves active filters. **Clear filters** returns to the full public list.
5. To rename a label, edit its name in **Categories and tags** and confirm. Its URL name and associations stay fixed. This immediately changes its display name wherever used, including published articles and historical snapshots. No content upload or republication is required for a shared label rename.

Creating a label does not select it for an article or automatically repair an imported name. After creating a missing label, choose its ID in the draft form and save (or re-import matching front matter). Duplicate names are allowed; ambiguous imported names block publication until selected/cleared explicitly. Names are matched case-insensitively during import; label lists use deterministic, case-sensitive name/ID order. Names are preserved as entered and must contain a non-whitespace character, with a maximum of 120 Unicode characters. URL names are unique within categories/tags and use lowercase ASCII letters, numbers, and hyphens, up to 160 characters. Label deletion and slug edits are unavailable.

A rename increments the label version and article versions for current draft/published associations, rejecting stale edits with 409. Live metadata changes update the article's update date; draft-only changes leave its public date unchanged. First publication dates and article URLs remain fixed. Historical-only associations show the renamed label without altering unrelated current article dates/versions. Saving the same name with a current version is a no-op. The dashboard reloads labels and the selected article after a successful change. If a response is uncertain, use **Refresh labels** and reload the article before retrying; duplicate slugs or stale versions return 409. Changes made in another browser may require a refresh too.

Public label lists contain only labels used by current public published revisions, even when an owner cookie is supplied. Unused/private/draft/history-only labels and admin versions are excluded. A private label filter does not reveal private article metadata: it returns an empty public result unless the same label is used by a public live article.

## Supported content and boundaries

- One `.md` file or a ZIP with exactly one article and its images. Markdown is UTF-8, at most 5 MiB, with optional YAML front matter up to 64 KiB. Title falls back to the first level-one heading, then the filename. New articles default to English; Chinese uses `language: zh`. Missing publication metadata remains a fixable draft issue.
- Recognized front matter: `title`, `summary`, `slug`, `language`, `written_at`, `category`, and `tags`. On replacement, omitted summary/language/writing date/category/tags inherit from the existing draft or published revision. Category/tag names resolve against existing labels; unresolved labels block publication. Use the dashboard to create labels and explicitly select or clear them in draft metadata.
- Shared React preview/public renderer: GFM tables, links, fenced code highlighting, inline/display LaTeX, and managed/HTTPS images. Raw HTML is skipped; Markdown never runs as MDX or JavaScript. KaTeX trust is disabled. External images display their host notice and load in the reader's browser with no referrer; the server never fetches them.
- ZIP limits: 50 MiB compressed, 100 MiB expanded, 200 files, and 400 total file/directory entries. Each image is at most 10 MiB and 16 million pixels; PNG, JPEG, GIF, and WebP bytes must decode and match their extension. Unused images are validated but create no asset row. Reject encrypted/split/ZIP64 archives, symlinks/special entries, duplicate names (including case-only duplicates), absolute/drive/backslash/traversal paths, and unsupported files such as SVG, `.DS_Store`, or executables. Entry paths are limited to 1024 UTF-8 bytes. Archive contents are read with size/CRC checks and are never extracted to user-chosen filesystem paths.
- Unavailable local images and forged managed-image URLs block publication. Publication also checks stored bytes against their recorded size/checksum. Metadata edits preserve the revision's image memberships. Re-importing normalized Markdown without its original ZIP does not grant membership based on copied URLs; upload the source article and images together.
- The private COS adapter, public search, and comments remain subsequent work. Permanent deletion, draft-only history browsing, and retention cleanup remain deferred.
- The layout is a neutral functional shell. Artistic direction, motion, responsive visual acceptance, keyboard interaction review, and interactive browser QA remain pending.

## Persistence, retries, and recovery

`UPLOAD_DIR` defaults to `backend/.data/uploads` when running Go from `backend/`. It must be accessible only to the API operating-system user (0700 directory, 0600 files). The API creates it if absent and rejects a broadly readable existing directory. Keep it outside any static file server. It contains **both staged uploads and retained image bytes**, including private and historical content. Do not delete it after imports finish. Stages use opaque `.upload` keys (older `.md` keys remain readable), and image objects use opaque `.asset` keys. SQL rollback/uncertain commits can leave private orphan files for later retention cleanup. The database stores Markdown snapshots, image metadata/memberships, and import records; storage keys are never returned by the API.

This is a **local, single-host development adapter**. Keep the directory across restarts together with its database. Do not deploy replicas with independent upload directories or ephemeral containers. A shared private COS adapter, retention cleanup, storage quotas, and coordinated database/image backup/restore verification are required before cloud use. No storage keys or cloud credentials are needed now, and no servers or buckets were changed.

An import returns `202` only after staging the file and committing a durable database job. The in-process worker claims queued jobs and commits the new draft plus successful job result atomically. An update checks the expected article version both at acceptance and completion. An intervening edit makes the import fail without overwriting that edit.

The worker holds a PostgreSQL advisory lock so only one consumer performs restart recovery. Queued jobs resume after restart. Previously processing jobs are marked failed with `worker_interrupted`; upload again with a new key after checking article state. Failed imports leave article state unchanged. Original stages and uncertain-commit stages are retained; automatic orphan/job/idempotency retention cleanup is deferred.

Imports, publish, and unpublish require UUID `Idempotency-Key` values. Successful operations retain their original status/body for 24 hours. Retry an uncertain result with the same key and payload; a different payload returns 409. A concurrent identical request may receive `request_in_progress` with `Retry-After`. Validation/rolled-back operations do not consume the key. Replaying an import returns its original queued acceptance; poll the returned job ID for current status. The dashboard retains uncertain keys and offers **Resume import** for accepted jobs during the current page session. Reloading discards that in-memory recovery state: inspect the article list before uploading again.

All management routes reuse owner session, exact-origin, and CSRF enforcement. Public article endpoints ignore owner cookies and return only public, published revisions. The asset route is the explicit exception: a validated owner session can read retained images; otherwise only current public revision memberships are readable. Invalid, expired, revoked, and ambiguous cookies never grant owner access. Image responses use validated MIME types, `nosniff`, same-origin resource policy, and no cache validators/304 shortcuts. Responses and SSR fetches are uncached. Making an article private cannot erase content someone already downloaded.

## Verification

Run the root README checks plus:

```bash
cd backend
export TEST_DATABASE_URL='postgres://test_role:test_password@127.0.0.1:5432/isaber_test?sslmode=disable'
go test -race -tags=integration ./internal/...
```

Use a disposable database whose role can create isolated schemas. Publishing tests cover real authenticated HTTP upload/preview/publish/private access, file-root reopening, immutable live revisions, stable publication dates and URLs, stale edits/imports, CSRF, failed imports, image blockers, ZIP path/size/type/CRC/pixel bounds, managed inline/reference rewrites, image membership across metadata edits and publication, retained image privacy, missing bytes, revoked sessions, history eligibility/cursor ties, metadata/image-preserving restore, stable live state/dates, source provenance, stale conflicts, label creation/assignment, name/ID pagination and cursor scoping, public-only label lists/filters, rename dates/versions, coordinated concurrent writes, and idempotency replay/conflicts. CI already runs all integration packages through its PostgreSQL service.

Local acceptance also exercised the production Next.js → Go proxy and queue against disposable PostgreSQL, validating real response payloads against OpenAPI and checking server-rendered code/math/images, unsafe markup filtering, revision isolation, and immediate privacy changes. The image acceptance check also uploads a valid ZIP larger than 10 MiB through the real Next.js rewrite and retrieves its exact PNG bytes through the Go authorization route. Next.js explicitly allows 51 MiB including multipart overhead; Go enforces the 50 MiB ZIP limit independently. History acceptance also exercises paginated history/detail/restore through the actual proxy, validates Revision/RevisionPage payloads against OpenAPI, and republishes a restored image-bearing snapshot before checking public SSR and image privacy. Label acceptance checks actual create/assign/rename operations, validates AdminLabel/AdminLabelPage/LabelPage payloads, and verifies public filtered SSR, article label links, stale conflicts, and private label exclusion. This is HTTP/SSR verification, not interactive browser or visual acceptance. GitHub CI and owner review remain pending for this branch.
