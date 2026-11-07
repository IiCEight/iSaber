# ADR 0008: Category/tag management and coordinated renames

Status: Implemented locally in `feat/article-labels`; pending review and merge.

## Decision

Use the existing category/tag tables and stable IDs/slugs for owner creation, renaming, draft assignment, and public browsing. Names support English and Chinese, preserve entered case/spacing, require a nonblank value, and are limited to 120 Unicode characters. Slugs use lowercase ASCII letters/numbers separated by hyphens, up to 160 characters. Different labels may share a name; their slugs remain unique within each kind. Imports resolve names only when unambiguous; explicit ID selection in the dashboard corrects unresolved imports. Creation does not assign a label automatically, and deletion remains deferred.

Renames change the shared display name. Revision memberships and label slugs stay fixed; historical label text follows the current name because snapshots store label IDs. Update the label version and versions/administrative modified times of articles whose current draft or published revision uses it in the same transaction. Update article public update dates only when the currently published revision uses the label, including private publications. Preserve first publication dates and article slugs. Historical-only associations do not change current article versions or live dates. An unchanged name with the current label version is a no-op; stale versions return 409.

Coordinate renames with article writes using PostgreSQL transaction advisory lock `7311171`. Article writes acquire a shared lock before their article row lock; new-article imports acquire it before creating associations. Rename acquires it exclusively before locking/updating the label and affected articles. This gives rename a stable affected-article set, while independent article writes can still run concurrently. All association writers must follow this order. Upgrade all Go writer instances before enabling this feature in a mixed-version deployment. The separate import-worker lock remains unchanged.

Public article detail reads use a read-only repeatable-read transaction so renamed label text and its article dates come from the same snapshot. Public label lists use one query over current public published memberships, ignore owner cookies, and omit admin versions. Draft-only, private-only, unused, and historical-only labels are absent.

Lists use ascending name and UUID order with PostgreSQL `COLLATE "C"`: case-sensitive UTF-8 byte order, which preserves Unicode code-point order without locale-dependent language grouping. Equal names use ID as the cursor tie-breaker. Bind cursors to category/tag kind and public/admin audience. Lists remain subject to concurrent changes between pages; refresh after a concurrent rename if needed.

## Consequences

No database migration or new deployment service is needed; schema version stays 4. The dashboard loads label pages for complete selectors and disables metadata editing until labels are available, preserving current assignments. Renaming refreshes the selected article so later edits use its new version. Public filters are server-rendered GET forms, and article labels link back to filtered public lists. The final artistic design remains pending owner references.

Creating and renaming labels follow the existing non-idempotent label contract. If a response is uncertain, refresh labels before retrying. A duplicate create slug or stale rename version returns 409 without silently changing another label. Cloud storage, deployment, retention, search, and comments remain separate work.
