# ADR 0004: PostgreSQL persistence and private object storage

Status: Selected for implementation in the persistence foundation branch; pending code review and merge.

## Context

The approved data model needs transactional publication, revision history, ownership constraints, owner sessions, and comment moderation. Images and staged uploads need private storage with article-aware authorization. Deployment targets include Tencent Cloud and JD Cloud, but no cloud database or bucket has been provisioned.

## Decision

- Use PostgreSQL, with major version 16 as the tested development/CI compatibility baseline. It is available in the Ubuntu development environment and supported by the intended cloud database family. Select the production service, region, and patch release during deployment preparation. Do not silently change major versions through an unqualified `latest` image.
- Use `pgx/v5` through Go's `database/sql` interface for a bounded connection pool and compatibility with Goose.
- Use embedded, versioned Goose SQL migrations and a dedicated `migrate up|status` operator command. Pin Go dependencies in `go.mod` and `go.sum`.
- Serialize migration commands with PostgreSQL advisory locks. Migrations are transactional and forward-only; the CLI offers no destructive down/reset operation.
- Keep migrations separate from API startup and require a separate `MIGRATION_DATABASE_URL`. Use `DATABASE_URL` for runtime access. Staging/production should use different migration and application roles.
- Report process liveness at `/healthz`; report readiness at `/readyz` only after a bounded database query confirms the required schema exists.
- Use private Tencent COS for production image objects and upload staging. The Go service will resolve internal storage keys and authorize asset delivery. No public bucket, permanent public object URL, or browser storage credentials are introduced.
- Defer the concrete object-storage adapter until the import slice needs it; its implementation will include a local development alternative. This branch defines the boundary and SQL metadata, not a functioning upload feature or a cloud connection.

## Consequences

The database enforces one owner, private-by-default articles, revision/asset ownership, valid lifecycle values, and basic limits. API authorization, snapshot immutability, publication validation, slug locking, version increments, and cache privacy remain service responsibilities and will be tested as those features are implemented.

Database outages or missing migrations result in a generic uncached `503` readiness response; liveness remains available. Driver errors and connection strings are not returned to clients. The API does not migrate tables as a side effect of a health check.

An older binary accepts a newer schema only under the project's forward-compatible migration policy. Breaking schema changes need a staged expand/migrate/contract rollout; a successful version query alone cannot prove arbitrary manual schema edits are compatible.

Development Compose publishes PostgreSQL on loopback with synthetic local-only credentials. CI uses an independent disposable PostgreSQL service. Real-cloud changes and secrets remain separate deployment tasks. No database server or COS SDK credentials are embedded in application images.

## References

- [PostgreSQL constraints](https://www.postgresql.org/docs/16/ddl-constraints.html)
- [pgx](https://github.com/jackc/pgx)
- [Goose provider and locking](https://pressly.github.io/goose/documentation/provider/)
- [TencentDB PostgreSQL version notes](https://intl.cloud.tencent.com/document/product/409/44365)

Provider availability varies by region and service configuration; verify the target before provisioning.
