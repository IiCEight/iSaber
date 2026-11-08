# Application container rehearsal

This milestone packages both applications and checks their connection before cloud deployment. It uses synthetic data and credentials. No registry publishing, server installation, TLS setup, or GitHub deployment secrets are needed. See [ADR 0011](../architecture/decisions/0011-container-foundation.md).

## Run on local Linux

Use Docker Engine with Compose v2. Run from the repository root with a dedicated project name to keep its data separate from development PostgreSQL:

```bash
export COMPOSE_FILE=infra/compose.rehearsal.yml
export COMPOSE_PROJECT_NAME=isaber-rehearsal
docker compose config --quiet
docker compose build
docker compose up -d --wait postgres
docker compose run --rm --no-deps migrate up
docker compose up -d --wait api frontend
docker compose exec -T frontend node --input-type=module < infra/tests/container-smoke.mjs
```

Open `http://127.0.0.1:3300` to inspect the empty public site. Only this loopback port is published; PostgreSQL and Go remain internal. This is an HTTP packaging rehearsal. The Go API uses production authentication with the synthetic origin `https://localhost`, so browser owner sign-in requires the later TLS setup. Continue using the normal [local development setup](../../README.md#local-development) for owner publishing acceptance.

The smoke script checks non-root frontend execution, database/schema readiness, uncached SSR, static JS/CSS delivery, the frontend API rewrite, anonymous owner-route rejection, and mutation Origin enforcement. CI also stops PostgreSQL and verifies that Go readiness returns 503. Repeat after restart to verify migration state is retained:

```bash
docker compose restart postgres api frontend
docker compose up -d --wait api frontend
docker compose run --rm --no-deps migrate up
docker compose exec -T frontend node --input-type=module < infra/tests/container-smoke.mjs
```

The second migration command should report zero new migrations. These checks establish packaging and connectivity, not a backup restoration exercise or browser acceptance of the publishing workflow.

## Images and persistent data

| Component | Packaging |
| --- | --- |
| Frontend | Node 24.22.2, Next standalone runtime, non-root user, port 3000 |
| Backend | Static Go 1.27.3 binaries, CA roots, UID/GID 65532, port 8080 |
| Operator tools | `/app/isaber-migrate` and `/app/isaber-owner` in the backend image; separate commands and runtime database variables |
| Database | PostgreSQL 16, dedicated rehearsal volume; no host port |
| Uploads and retained images | Private `/data/uploads`, dedicated volume; never mounted into the frontend |

Go's image health check uses `/app/isaber-probe`; it checks database/schema readiness at `/readyz`. Frontend health checks `/healthz` for process liveness. The frontend image embeds `http://api:8080` as its API rewrite target. Keep that DNS service name in each environment so the same image can be promoted unchanged. Do not override `BACKEND_ORIGIN` only at runtime: that would change SSR calls without changing the built rewrite.

The API image is shell-free. Use explicit `--entrypoint /app/isaber-migrate` or `--entrypoint /app/isaber-owner` when invoking operator tools outside Compose. Inject migration/owner database connections only into those one-off commands. Production must use appropriate separate roles. Never place real credentials in Dockerfiles, build arguments, or this rehearsal Compose file.

## Stop and inspect

```bash
docker compose ps
docker compose logs --tail 100
docker compose down
```

`down` retains named volumes. After reviewing that this is the disposable rehearsal project, the following removes its synthetic database and image bytes:

```bash
docker compose down --volumes
```

Do not use that cleanup command against real staging/production data. Database and media must be backed up and restored together before live hosting.

## GitHub exercise

Push this milestone branch and open one PR. In its Checks tab, inspect **Containers**, alongside the existing API contract, Frontend, and Backend jobs. The new job runs the isolated build/migrate/start/smoke/restart sequence above on a GitHub runner and removes its own volumes afterward. No cloud server participates. Once it passes, consider adding the actual **Containers** check to branch protection.

Next: inspect server capacity/workloads and registry access, choose private storage and database hosting, then prepare TLS, deployment identity, backups, and staging delivery with guided commands. Packaging success alone is not permission to deploy production.

## Local validation record

On 2026-11-08, separate application image builds and the complete Compose sequence passed locally using Podman's Docker-compatible API and Compose v5.7.0. This included explicit migration to schema 5, non-root/read-only application startup, SSR/static assets, real API routing and authorization, 503 readiness during database loss, restart, and zero new migrations with the retained database. The disposable stack and volumes were removed afterward. Frontend types/lint/build and all eight existing HTTP smoke tests passed; backend unit tests and static analysis passed in the pinned build environment. PR #11 and its main run subsequently passed all four GitHub checks, including Containers on the Docker engine. Interactive browser acceptance remains pending.
