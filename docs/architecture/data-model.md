# Data model

Status: Approved logical data model. PostgreSQL migrations, authentication, Markdown/ZIP imports and authorized images, published revision history, labels/search and moderated guest comments are merged and deployed. The optional revision last updated time requires schema version 6. See the [publishing guide](../operations/publishing.md) and [database guide](../operations/database.md).

Source: [approved PRD](../product/requirements.md). Contract: [OpenAPI](../../contracts/openapi.yaml). Behavior and HTTP conventions: [API design](api-design.md).

## 1. Design principles

- One owner, no reader accounts. The owner is provisioned through an operational command.
- An article is a stable identity; revisions hold its Markdown and editorial metadata.
- A draft never replaces the published revision until explicit publication succeeds.
- Access rules depend on current article state, not an object's filename or storage location.
- Go owns validation, publication transactions, and authorization. PostgreSQL and private COS are selected in [ADR 0004](decisions/0004-postgresql-and-private-storage.md); the COS adapter follows in the import slice.

Identifiers are UUIDs. Store event timestamps in UTC and serialize them as RFC 3339 timestamps. Store `written_at` as a calendar date; display event times in Asia/Shanghai. Database column naming below describes the logical model, not a finalized physical schema.

## 2. Entities

| Entity | Main fields | Constraints and purpose |
| --- | --- | --- |
| Owner | `id`, `username`, `password_hash`, `created_at`, `credentials_changed_at` | Exactly one provisioned owner; no registration API. Username/password authentication is approved; hashing parameters remain an implementation decision. Never expose the hash. |
| Session | `id`, `owner_id`, `token_hash`, `csrf_token_hash`, `created_at`, `last_seen_at`, `expires_at`, `revoked_at` | Store hashes, not bearer secrets. Sign-out and credential recovery revoke sessions. Expiry and password-hashing parameters require implementation review. |
| Article | `id`, `slug`, `visibility`, `comments_enabled`, `draft_revision_id`, `published_revision_id`, `published_at`, `updated_at`, `version`, `created_at`, `modified_at` | Stable identity and global access controls. Unique non-null slugs; incomplete imports may have no slug. `version` increments on every article mutation. |
| Revision | `id`, `article_id`, `markdown`, `title`, `summary`, `language`, `written_at`, `updated_at`, `category_id`, `created_at`, `first_published_at`, `restored_from_revision_id` | Immutable editorial snapshot. Nullable `updated_at` overrides automatic publication dating. Incomplete drafts allow missing required publication fields. Each snapshot belongs to exactly one article. |
| RevisionTag | `revision_id`, `tag_id` | Unique pair. Tags remain associated with their snapshot. |
| Category | `id`, `name`, `slug`, `version` | Stable unique slug; editable display name. Category deletion is deferred. |
| Tag | `id`, `name`, `slug`, `version` | Stable unique slug; editable display name. Tag deletion is deferred. |
| Asset | `id`, `article_id`, `storage_key`, `media_type`, `byte_size`, `checksum`, `created_at` | Private storage object, immutable bytes. Accepted types: PNG, JPEG, GIF, WebP. Storage keys are never public API fields. |
| RevisionAsset | `revision_id`, `asset_id`, `original_path` | Asset and revision must belong to the same article. Records attachment membership independently of storage deduplication. |
| ImportJob | `id`, `article_id`, `expected_version`, `upload_storage_key`, `status`, `result_revision_id`, `issues`, `created_at`, `finished_at` | Retains the private staged upload for processing. Validates it and atomically creates a draft or replaces the current draft pointer. On failure, existing article state is unchanged. Never expose the storage key. |
| Comment | `id`, `article_id`, `display_name`, `body`, `status`, `version`, `created_at`, `moderated_at` | `pending`, `approved`, `rejected`, or `removed`. Plain text; no email or reader identity. Comments attach to the article, not a revision. |
| IdempotencyRecord | `scope`, `key`, `request_hash`, `state`, `response_status`, `response_body`, `created_at`, `expires_at` | Prevent duplicate successful imports, publication operations, and comment submissions on retry. Never persist login credentials here. |

Operational logging records action identifiers and outcomes, not article bodies, passwords, session tokens, or comment text. A full audit-history product is outside this contract.

## 3. Article state and invariants

`status` is derived: `published` when `published_revision_id` is set; otherwise `draft`. A previously published article can therefore be draft/unpublished while retaining its first publication timestamp and history. `has_draft` is independent of `status`.

| Operation | Result |
| --- | --- |
| Import a new article | Create identity with private visibility, comments enabled, no published revision, and one draft snapshot. Missing required metadata is reported for correction. |
| Import into an existing article | Stage assets and a new snapshot, then replace only its draft pointer after checking `expected_version`. Preserve live content and existing controls. |
| Edit draft metadata | Create a new immutable snapshot and advance the draft pointer. No raw Markdown editor is required; content replacement occurs by upload. |
| Publish | Validate the current draft and its assets, switch the published pointer, clear the draft pointer, increment version, and update public dates atomically. |
| Unpublish | Clear the published pointer, preserving its snapshot as a draft if no newer draft exists. Keep first publication time and history. Repeating an unpublish on an already unpublished article is a no-op. |
| Restore history | Copy a previously published snapshot into a new draft. Preserve article slug, visibility, controls, dates, comments, and current live content. |
| Change visibility | Apply immediately to the article, including all content and media access paths. It does not publish a draft. |
| Change comment setting | Apply immediately to guest submissions. Previously approved comments remain readable on public published articles. |

- A public reader may access only a public article's current published revision. Drafts and history require owner access.
- A published and draft pointer must reference snapshots of that same article. Enforce this in database constraints and transactional application validation.
- The first publication time is set once, including if the first publication is private. It is never reset by unpublishing, visibility changes, or revision publication.
- Article `updated_at` uses the published revision’s optional owner-selected time, which may predate first publication. Without that override, it represents the latest change to live article content or metadata. Draft saves change `modified_at` and `version`, not article `updated_at`. Publishing applies the draft’s selected time or the current time. Live visibility/comment changes preserve an explicit timestamp, otherwise they use the current time; unpublishing records its transition. Public dates are absent before first publication.
- Slug edits are permitted only before the article has ever been published. Re-uploaded front matter cannot silently change an existing article's slug or global controls.
- Revision category/tag links refer to stable identities. Label renames preserve associations and update automatic `updated_at` for affected live articles while preserving explicitly selected timestamps; prior snapshot label text is not preserved.
- A draft snapshot can be incomplete. Publication requires a title, unique slug, language, writing date, valid category/tag references, and resolved local assets.
- No permanent article, category, or tag deletion API is included in this first release.

## 4. Assets and imports

The upload endpoint accepts a multipart file and optional existing article identity/version. It creates an import job; the owner polls its result. This asynchronous contract avoids keeping an HTTP request open while the archive is validated and images are stored. A durable job runner can operate within the Go deployment; no separate queue service is required by this design.

1. Validate request limits and the archive; reject unsafe paths and expansion before committing content.
2. Parse exactly one UTF-8 article, resolve relative image paths, and stage validated images in private storage.
3. Preserve external HTTPS image links without fetching them. Record warnings for external image requests and invalid math.
4. Atomically save the article/draft snapshot and asset associations after verifying the expected article version.
5. Mark the job succeeded with article and revision IDs. If any step fails or the article changed during processing, mark the job failed and keep existing draft/live state unchanged.

An import can succeed with a non-publishable draft, such as a missing writing date or image. Return structured blocking issues and warnings. Unsupported archives and files fail the job. Metadata edits recompute draft issues. To fix missing images, re-import a corrected package into the same article.

Managed Markdown image URLs use `/api/v1/articles/{article_id}/assets/{asset_id}`. That route checks current access on every request. Anonymous access requires that the asset belong to the current live revision of a public article; an authenticated owner may retrieve assets from that article's other retained revisions. History does not create a public storage URL. Do not deduplicate asset identities across articles in the first implementation.

Use private storage and authenticated application delivery. Approved initial policy: `Cache-Control: no-store` for article data, media, previews, search, and management responses. This prioritizes immediate privacy transitions over public caching. Any later CDN or page cache requires an explicit invalidation/access-control design. Next.js rendering must follow the same policy.

Staged objects are not public. Retry failed jobs safely; abandon unfinished work with a clear error after restart if resumption is unsupported. Cleanup may remove only unreferenced staged objects, never assets referenced by retained revisions. Back up both object bytes and their database associations.

## 5. Concurrency, retries, and transactions

Owner mutations include `expected_version` for the target article, label, or comment. A stale value produces `409 version_conflict` with no changes. Creation does not require a prior version. Imports check versions again when they commit, not only when the job is accepted.

Renaming a label also increments versions of affected live articles when their metadata dates change, so an in-progress owner edit cannot silently overlook that change. Current draft-only associations also increment article versions/administrative modified times without changing live dates. Shared/exclusive transaction coordination and public snapshot reads are described in [ADR 0008](decisions/0008-label-renames.md).

Publish/unpublish, revision restoration, imports, and guest comment submission require `Idempotency-Key`. For the approved 24-hour retention period, the same key and semantic payload replay the original result, even if its expected version has since changed. A different payload using that key returns `409 idempotency_conflict`. An in-progress duplicate returns `409 request_in_progress` with `Retry-After`. Re-authenticate and authorize before replaying owner responses.

Scope keys by authenticated owner and operation/target, or by public comment operation and article. Unpredictable client-generated UUID keys protect against accidental collisions; key possession is not an authorization method. Pending/rejected comment content is never returned by the public replay response.

Transactions must protect publication pointers, dates, revision/asset membership, and idempotency success records together. An import job and its replay record must be committed together before acknowledging acceptance. Storage writes are staged outside database transactions and exposed only through committed associations.

## 6. Comments, taxonomy, and search

- Public comments are visible only when approved and their article is currently public and published. No anonymous route returns moderation details.
- Moderation removal is a logical state change. Physical retention/purging is a later operational policy, with privacy obligations reviewed before launch.
- Public category/tag lists include only labels used by public published revisions. Admin lists include all labels. Draft-only and private-only labels are not discoverable publicly.
- Search reads only current public published revisions and their labels. Draft imports do not change search results. English case-insensitive word matching and Chinese literal substring matching follow the PRD.
- Prefer database queries for the first release; no external index is assumed. Changes must be visible on subsequent requests. Define indexes and query performance once expected volumes and the database choice are reviewed.

## 7. Approval and remaining implementation decisions

The owner approved revision pointers, immediate visibility controls, public date semantics, asynchronous imports, incomplete drafts, version checks, retry semantics, owner sessions, and the no-store delivery policy.

Physical database/storage selection, password-hashing parameters, session expiry, rate-limit thresholds, and code-generation tooling still require implementation decisions. Performance budgets remain open. This approval does not introduce migrations or approve a visual design.
