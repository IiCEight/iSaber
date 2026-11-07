# Markdown publishing

Status: The first publishing slice is merged in PR #6. ZIP/image imports and authorized local image delivery are implemented in `feat/typora-image-imports`, pending GitHub review. Cloud deployment and the final artistic design remain separate work.

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

## Supported content and boundaries

- One `.md` file or a ZIP with exactly one article and its images. Markdown is UTF-8, at most 5 MiB, with optional YAML front matter up to 64 KiB. Title falls back to the first level-one heading, then the filename. New articles default to English; Chinese uses `language: zh`. Missing publication metadata remains a fixable draft issue.
- Recognized front matter: `title`, `summary`, `slug`, `language`, `written_at`, `category`, and `tags`. On replacement, omitted summary/language/writing date/category/tags inherit from the existing draft or published revision. Category/tag names resolve against existing labels; unresolved labels block publication. This dashboard can clear them; label creation/selection UI follows later.
- Shared React preview/public renderer: GFM tables, links, fenced code highlighting, inline/display LaTeX, and managed/HTTPS images. Raw HTML is skipped; Markdown never runs as MDX or JavaScript. KaTeX trust is disabled. External images display their host notice and load in the reader's browser with no referrer; the server never fetches them.
- ZIP limits: 50 MiB compressed, 100 MiB expanded, 200 files, and 400 total file/directory entries. Each image is at most 10 MiB and 16 million pixels; PNG, JPEG, GIF, and WebP bytes must decode and match their extension. Unused images are validated but create no asset row. Reject encrypted/split/ZIP64 archives, symlinks/special entries, duplicate names (including case-only duplicates), absolute/drive/backslash/traversal paths, and unsupported files such as SVG, `.DS_Store`, or executables. Entry paths are limited to 1024 UTF-8 bytes. Archive contents are read with size/CRC checks and are never extracted to user-chosen filesystem paths.
- Unavailable local images and forged managed-image URLs block publication. Publication also checks stored bytes against their recorded size/checksum. Metadata edits preserve the revision's image memberships. Re-importing normalized Markdown without its original ZIP does not grant membership based on copied URLs; upload the source article and images together.
- The private COS adapter, revision browsing/restoration, public search, label management, and comments remain subsequent work. Stored snapshots preserve the foundation for history; there is no history UI yet.
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

Use a disposable database whose role can create isolated schemas. Publishing tests cover real authenticated HTTP upload/preview/publish/private access, file-root reopening, immutable live revisions, stable publication dates and URLs, stale edits/imports, CSRF, failed imports, image blockers, ZIP path/size/type/CRC/pixel bounds, managed inline/reference rewrites, image membership across metadata edits and publication, retained image privacy, missing bytes, revoked sessions, and idempotency replay/conflicts. CI already runs all integration packages through its PostgreSQL service.

Local acceptance also exercised the production Next.js → Go proxy and queue against disposable PostgreSQL, validating real response payloads against OpenAPI and checking server-rendered code/math/images, unsafe markup filtering, revision isolation, and immediate privacy changes. The image acceptance check also uploads a valid ZIP larger than 10 MiB through the real Next.js rewrite and retrieves its exact PNG bytes through the Go authorization route. Next.js explicitly allows 51 MiB including multipart overhead; Go enforces the 50 MiB ZIP limit independently. This is HTTP/SSR verification, not interactive browser or visual acceptance. GitHub CI and owner review remain pending for this branch.
