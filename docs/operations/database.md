# Database setup and migrations

Status: PostgreSQL foundation is merged for local/CI use. Owner authentication follows in its own branch using the existing identity tables. Article APIs and cloud deployment remain subsequent slices; no cloud data has been changed.

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

The Go pool allows five open connections and two idle connections initially. Both are modest development defaults; review aggregate pool sizes against the database limit before adding replicas or migration workers.

## Integration tests

Use only a disposable local/test database whose role can create schemas:

```bash
cd backend
export TEST_DATABASE_URL='postgres://isaber_dev:isaber_local_only@127.0.0.1:5432/isaber?sslmode=disable'
go test -race -tags=integration ./internal/migrations -v
```

Each test creates a uniquely named schema, sets its own search path, and drops only that schema afterward. Tests never apply migrations to the database's default schema. A missing test URL fails explicitly rather than skipping the integration checks. The CI backend job provisions its own PostgreSQL service and runs the same tests.

Tests cover upgrade from the identity migration while preserving existing data, repeated migration runs, readiness before/after migration, cross-article revision and asset rejection, the single-owner constraint, private article defaults, and pending-comment/validation constraints. Authentication adds its own integration tests; run `go test -race -tags=integration ./internal/...` to include them. Publication is not implemented yet.

## Migration authoring and release policy

1. Add a new sequential SQL file; never edit a migration already released to an environment.
2. Keep changes transactional and backward-compatible with the currently deployed application. Increase `RequiredVersion` when a new binary depends on the schema change.
3. Test both a fresh install and upgrading existing data, then validate the old/new application compatibility required for rollout.
4. Before changing cloud data, verify a restorable backup and select the correct environment's migration credentials.
5. Run exactly one migration phase per release; the advisory lock prevents overlapping Goose migration processes, but does not replace release coordination.
6. Start/promote application versions only after migration and health verification. Do not automatically roll back database data if an application rollout fails.

There is no automatic down/reset command. CLI errors deliberately avoid raw SQL row data and connection URLs; use an authorized database console and sanitized logs to diagnose availability, permissions, schema history, or failed constraints.

In staging/production, use a schema-owning migration role and a runtime role with only required table DML and read access to migration version metadata. The runtime role does not need database/schema creation or table alteration privileges. Prefer certificate-verified TLS (`sslmode=verify-full`) for remote connections; configure CA trust for the selected service. Disabling TLS in the examples is for the loopback development service only.

## Image storage boundary

The SQL schema stores private object keys and revision membership, not image bytes or public COS URLs. The import slice will implement the storage adapter and authorized Go delivery route described by the API contract. This branch does not create buckets, fetch external images, or require Tencent/JD API credentials.

Before enabling image uploads, select separate staging/production buckets, define cleanup for abandoned staged uploads, verify database-plus-object restoration, and test article visibility changes against every media access route. Storage readiness can be added when the running application actually depends on that adapter.
