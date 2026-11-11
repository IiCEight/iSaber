# Database setup and migrations

Status: PostgreSQL and migrations are deployed on Tencent in the isolated blog Compose project. Schema version 5, separate runtime/operator roles and private persistent storage are verified. See [deployment](deployment.md) for the accepted installation; the commands below cover local development and deliberate migration operations.

## Local development

Use Docker Compose to start the local database from the repository root:

```bash
docker compose -f infra/compose.dev.yml up -d --wait
```

This creates a loopback-only PostgreSQL 16 service on port 5432 with a named volume. The documented credentials are synthetic local development values. Do not reuse this Compose configuration for publicly reachable or production databases.

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
