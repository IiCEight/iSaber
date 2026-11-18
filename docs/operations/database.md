# Database setup and migrations

Status: Local development is at schema 10 after the additive Friends migration on November 18, 2026. A private local backup preceded the upgrade. Friends has not been deployed. Tencent was upgraded to schema 9 for Category pages on November 17, 2026; its owner, content, assets and existing application remained intact. The production runtime/operator roles and private persistent storage are separate. See [Friends operations](friends.md) for migration 10 release requirements and [deployment](deployment.md) for the accepted Tencent setup. The commands below cover local development and deliberate migration operations.

## Local development

Use Docker Compose to start the local database from the repository root:

```bash
docker compose -f infra/compose.dev.yml up -d --wait
```

This creates a loopback-only PostgreSQL 16 service on port 5432 with a named volume. The documented credentials are synthetic local development values. Do not reuse this Compose configuration for publicly reachable or production databases.

The current Linux workspace uses the already-installed Podman runtime instead of Docker Compose. Its persistent development database was created on November 16, 2026:

- Container: `isaber-dev-postgres`, PostgreSQL 16.
- Named volume: `isaber-dev-postgres-data`, mounted at `/var/lib/postgresql/data`.
- Address and credentials: the loopback-only `127.0.0.1:5432` connection in `backend/.env.example`.
- State: schema 10 as of November 18, 2026; currently no local owner, articles, or categories. No production data has been copied.

Reuse it with `podman start isaber-dev-postgres`; stop it while retaining data with `podman stop isaber-dev-postgres`. The Compose setup above is an alternative installation using the same port. Local initialization does not copy the production account or articles.

In the terminal where you will run Go:

```bash
cd backend
set -a
. ./.env.example
set +a
go run ./cmd/migrate status
go run ./cmd/migrate up
go run ./cmd/migrate status
go run ./cmd/api
```

The example file contains development configuration only and may be sourced as shown. Do not source an untrusted downloaded file as shell code. Go does not automatically load `.env` files. For real secrets, inject environment variables through the deployment secret mechanism rather than putting connection URLs into committed files or shell history.

`status` shows Goose migration bookkeeping and can initialize its version table on a new database; it does not apply application migrations. `up` applies pending embedded migrations under a PostgreSQL advisory lock. Running `up` again after success reports zero pending changes.

The runtime API requires `DATABASE_URL`. The migration command requires `MIGRATION_DATABASE_URL` and intentionally does not fall back to the runtime variable. The two example URLs use the same local superuser solely for development convenience; production roles must be separate.

Check these URLs in another terminal:

```bash
curl -i http://127.0.0.1:8080/healthz
curl -i http://127.0.0.1:8080/readyz
```

`/healthz` reports that the API process is running. `/readyz` returns `200` when the database is reachable and its schema is migrated, or a generic `503` when it is not. Readiness is bounded to two seconds and never returns raw connection errors or credentials. A failed database connection does not authorize or simulate any business endpoint.

To stop local PostgreSQL while retaining data:

```bash
docker compose -f infra/compose.dev.yml stop
```

The container image is pinned to major version 16 so compatible patch updates can be pulled deliberately. A major upgrade needs its own backup and upgrade plan. Do not delete the named volume to address migration failures.

## Schema layout

| Migration | Tables | Important constraints |
| --- | --- | --- |
| `00001_identity.sql` | `owners`, `sessions` | At most one owner, unique session token hashes, token hash sizes, session expiry |
| `00002_content.sql` | `categories`, `tags`, `articles`, `revisions`, `revision_tags`, `assets`, `revision_assets` | Private default, unique slugs, positive versions, article-owned revisions/assets, accepted image types/sizes |
| `00003_delivery.sql` | `import_jobs`, `comments`, `idempotency_records` | Job result ownership, pending comments, plain-text field limits, scoped idempotency keys and completion state |

Foreign keys ensure that article pointers, restored revision references, and revision assets cannot cross article ownership. They do not replace reader authorization. Incomplete drafts are permitted; publication completeness is validated by the future publishing service.

Migration `00004_revision_import_issues.sql` adds internal saved revision issues and the original import filename. The comment milestone adds `00005_comment_rate_index.sql` for creation-time quota lookups and requires schema version 5. Apply it with `migrate up` before starting the new binary; it preserves existing rows.

Migration `00006_revision_updated_at.sql` adds a nullable revision timestamp for owner-selected last updated times. The last-updated-time release required schema version 6. Apply `migrate up` before starting its binary. Existing revisions receive null and retain automatic publication dating; existing article dates are unchanged. The additive column remains compatible with older binaries.

The subsequent remembered-login release requires schema version 7. Migration `00007_remembered_sessions.sql` adds `sessions.remember_me`, defaulting to false. Existing sessions keep their expiry, idle policy, and credential hashes; no article data changes. Apply `migrate up` with operator credentials before deploying that API binary. The API does not apply migrations automatically. Older binaries can still use the additive schema. It was applied to production on November 15, 2026 with the reviewed migration binary in the existing migration container under the deployment lock. Owner, article, revision, asset, and session credential/expiry fingerprints matched before and after. Runtime SELECT/INSERT/UPDATE access to the new column and continued HTTPS health were verified.

The global-pinning release requires schema version 8. Migration `00008_article_pins.sql` adds `articles.pinned`, defaulting to false, a publication constraint, a browse-order index, and a trigger that clears pins when unpublishing. Existing articles, revisions, visibility, and dates remain unchanged. The trigger also allows the preceding binary to unpublish a pinned article during rollout. Apply it with operator credentials before starting the new API. On November 16, 2026, the owner authorized migration of both environments. The persistent local database was initialized through migration 8. Tencent advanced from schema 7 to 8 under the deployment lock using the checksum-verified migration binary from backend commit `d8de95b`; a private pre-migration dump and operation log were retained on that host. Existing data fingerprints, runtime column privileges, the pin constraint/index/trigger, unchanged application container IDs, and HTTPS health were verified. See the [deployment record](deployment.md#global-pinning-migration--november-16-2026) for the retained audit location.

The Go pool allows five open connections and two idle connections initially. Both are modest development defaults; review aggregate pool sizes against the database limit before adding replicas or migration workers.

## Integration tests

Use only a disposable local/test database whose role can create schemas:

```bash
cd backend
export TEST_DATABASE_URL='postgres://isaber_dev:isaber_local_only@127.0.0.1:5432/isaber?sslmode=disable'
go test -race -tags=integration ./internal/migrations -v
```

Each test creates a uniquely named schema, sets its own search path, and drops only that schema afterward. Tests never apply migrations to the database's default schema. A missing test URL fails explicitly rather than skipping the integration checks. The CI backend job provisions its own PostgreSQL service and runs the same tests.

Tests cover upgrade from the identity migration while preserving existing data, repeated migration runs, readiness before/after migration, cross-article revision and asset rejection, the single-owner constraint, private article defaults, and pending-comment/validation constraints. Authentication adds its own integration tests; run `go test -race -tags=integration ./internal/...` to include them. Comment tests additionally cover guest admission, quota/replay/privacy behavior and owner moderation. Publishing integration tests cover draft/live isolation, version conflicts, retries, private access, and failed imports.

## Migration authoring and release policy

1. Add a new sequential SQL file; never edit a migration already released to an environment.
2. Keep changes transactional and backward-compatible with the currently deployed application. Increase `RequiredVersion` when a new binary depends on the schema change.
3. Test both a fresh install and upgrading existing data, then validate the old/new application compatibility required for rollout.
4. Before changing live data, review the migration impact and any manual export/recovery plan, and select the correct operator credentials. Automated backups are outside the revised delivery scope.
5. Run exactly one migration phase per release; the advisory lock prevents overlapping Goose migration processes, but does not replace release coordination.
6. Start/promote application versions only after migration and health verification. Do not automatically roll back database data if an application rollout fails.

There is no automatic down/reset command. CLI errors deliberately avoid raw SQL row data and connection URLs; use an authorized database console and sanitized logs to diagnose availability, permissions, schema history, or failed constraints.

In production, use a schema-owning migration role and a runtime role with only required table DML and read access to migration version metadata. The runtime role does not need database/schema creation or table alteration privileges. Prefer certificate-verified TLS (`sslmode=verify-full`) for remote connections; configure CA trust for the selected service. The local examples and the single-host Compose private database network use `sslmode=disable`; no database port is published by the production project.

## Image storage boundary

The SQL schema stores private object keys and revision membership, not image bytes or public COS URLs. The image slice implements authorized Go delivery from local private storage; a private COS adapter remains separate storage work. The Markdown/image implementation stores private uploads and retained image bytes locally (see the [publishing guide](publishing.md)) and does not create buckets, fetch external images, or require Tencent/JD API credentials.

The proposed single-host deployment keeps database and uploads in separate private persistent volumes. Accept their persistence and media authorization before cutover; see [deployment](deployment.md). A future COS adapter needs its own private-bucket and authorization acceptance. Automated backups/retention are outside the revised delivery scope.

## Category pages migration (local implementation)

The category-pages release requires schema 9. `00009_category_pages.sql` adds empty English/Chinese category descriptions and an independent `articles.category_pinned` flag, default false. The trigger clears that pin on unpublishing or publishing into another category, including pointer updates by the preceding binary. Same-category replacements retain it. Global pins and content dates are preserved. Description changes use category version checks and do not update article dates or versions.

This migration is tested against an isolated PostgreSQL database, including upgrade from schema 8 and repeated application. On November 17, 2026, it was applied to the existing local `isaber-dev-postgres` container after confirming its loopback-only port and persistent volume. A custom-format backup, restore-list validation, migration log, and before/after fingerprints were retained in the ignored private directory `backend/.data/backups/category-pages-20261117/20261117T120700620751Z/`. All pre-existing rows were preserved; the database had zero owners, articles, and categories. The new defaults and trigger were verified, and a second run applied zero migrations. The local API runs on port 8080 and the frontend on port 3000, with uploads in `backend/.data/uploads`. The prior sample preview on port 3100 and sample API on port 8089 were stopped. No cloud migration has been applied for this feature. Apply it with operator credentials before starting the new cloud API; retain the normal backup, deployment lock, and verification process.
