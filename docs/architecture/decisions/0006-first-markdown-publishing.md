# ADR 0006: Draft-first Markdown publishing

Status: Accepted and merged in PR #6. Subsequent ZIP/image behavior is recorded in ADR 0007.

Operational update, November 11, 2026: [ADR 0019](0019-simple-delivery.md) revises the earlier staging/automated-backup requirements. The initial Tencent deployment uses a private persistent uploads volume; COS remains separate work. The original milestone rationale follows.

## Decision

Deliver the first complete author workflow with `.md` uploads, metadata correction, a shared preview/reading renderer, and explicit publish/unpublish. Keep the approved full API contract; document this implemented subset rather than claiming ZIP, image storage, search, comments, or revision restoration are complete.

Go stages uploads in a private local directory and commits PostgreSQL queue records before returning acceptance. An in-process worker creates immutable revision snapshots and atomically advances only the draft pointer. Publication switches the live pointer in a version-guarded transaction. Public reads resolve only the current public published revision, including when an owner cookie is present.

Persist unresolved import issues with their revision so later reads cannot forget blocking metadata problems. Migration 00004 adds internal `revisions.source_issues` and `import_jobs.filename`; neither is a new public response field. Only explicit metadata edits clear their corresponding saved issues. Local image references are recomputed from the Markdown AST and remain blocking until managed image support exists.

Use Goldmark for backend Markdown inspection and YAML v3 for front matter, rejecting aliases and malformed metadata. React Markdown, remark GFM/math, rehype KaTeX, and rehype highlight provide the same rendering component for previews and published pages. Skip raw HTML and disable KaTeX trust. Do not run MDX or fetch external resources in Go.

Use transactionally stored UUID idempotency keys for accepted imports and publication operations. Replays return the stored response representation, even after the article version changes. Check optimistic versions inside article row locks. Invalid/stale operations roll back without consuming a key. Store retry records for 24 hours.

## Consequences

The owner can prove the core workflow before final visual design and cloud storage work. First publication dates and URLs stay stable; uploading a replacement cannot change live content or visibility. Preview is a UI review step; API publication independently validates the draft rather than trusting a client-side flag.

The local filesystem adapter is for one host with persistent staging storage. It is not the selected production storage architecture from ADR 0004. Private COS, attachment validation/delivery, retention cleanup, and backup recovery remain necessary before deployment. Keep both the database and staged uploads across local restarts. One worker holds a database advisory lock; interrupted processing jobs fail explicitly while queued jobs resume.

Read the [publishing guide](../../operations/publishing.md) for supported content, recovery behavior, tests, and deferred work.
