# ADR 0007: Typora ZIP imports and private image delivery

Status: Accepted and merged in PR #7.

Operational update, November 11, 2026: [ADR 0019](0019-simple-delivery.md) revises the earlier staging/automated-backup requirements. The proposed single-host private uploads volume is pending production storage acceptance; COS remains separate work. The original milestone rationale follows.

## Decision

Accept one Markdown file or a standard ZIP containing one article and its PNG/JPEG/GIF/WebP images. Validate archive names, size/count limits, CRCs, image formats/extensions, and decoded pixel bounds before changing article state. Never extract archive paths to disk. The parser locates image destinations, including reference definitions, and rewrites only their source spans to article/asset API URLs. Code examples and unrelated Markdown formatting remain intact.

Use the existing `assets` and `revision_assets` tables; schema version remains 4. Each referenced ZIP image creates immutable private bytes and an article-owned asset. Repeated references share its membership within that import. Metadata snapshots copy memberships. Draft imports and asset rows commit atomically; a failed import preserves the article and may leave private orphan bytes for later cleanup. Opaque image keys, filenames, and checksums are internal fields rather than additions to the Revision API schema.

A managed-looking URL is insufficient for publication: it must match the revision's recorded asset membership and owning article. Publication also verifies stored bytes against their size/checksum. A replacement source package must include its image files; copying normalized URLs does not infer attachments or cross article ownership boundaries.

Implement the contracted image route through Go. Anonymous visitors can read only assets attached to the current published revision of a public article. A valid owner session may read images in retained revisions. Invalid/revoked/ambiguous credentials cannot grant owner access. Article list/detail endpoints continue to ignore owner cookies for privacy. Image responses have validated content types, no-store, nosniff, same-origin resource policy, and no validators or public object redirects.

## Local storage and deployment boundary

Continue using the private local `UPLOAD_DIR` for development. It now contains both staged uploads and retained image bytes and must persist together with PostgreSQL. Files and directory entries are synced before SQL can expose them; files use 0600 permissions, and the root directory must be private. Unreferenced/uncertain-commit files are retained to avoid unsafe deletion.

The local adapter does not replace ADR 0004's private COS production choice. COS setup, retention/quotas, coordinated backups, and restore verification remain required before deployment or replicas. This slice creates no cloud resources and requires no cloud credentials. Next.js's proxy limit is explicitly raised above the ZIP limit to pass the upload through; Go remains authoritative for all size and content checks.

Read the [publishing guide](../../operations/publishing.md) for packaging examples, limits, recovery, verification, and current deferred work.
