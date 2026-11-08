# ADR 0011: Separate application images and an isolated delivery rehearsal

Status: Accepted; merged in PR #11 with successful PR/main CI, including Containers. Cloud deployment remains pending.

## Context

Publishing and guest moderation are merged through PR #10. Before connecting GitHub to cloud servers, we need reproducible application packaging and evidence that the packaged frontend can reach the packaged API. Artistic design and the private COS adapter remain separate work.

## Decision

Build separate frontend and backend images from their respective directories, with allowlisted Docker contexts excluding environment files, private uploads, Git metadata, and local dependencies. Pin base-image version tags to the existing Node/Go toolchains. Resolve and record immutable application image digests when registry delivery is introduced; version tags alone do not guarantee immutable base contents.

Use Next.js standalone output with its traced runtime dependencies and static assets. The image runs as the Node image's non-root user. Its API rewrite is baked at build time to `http://api:8080`. Both staging and production must provide the internal DNS alias `api`; do not override only the runtime origin and create different SSR/proxy targets. Site-specific public origins and authentication secrets belong to Go runtime configuration, never frontend build arguments. A future hosting topology change requires reviewing this internal endpoint convention.

Compile static Go binaries for API, migrations, owner provisioning/recovery, and a readiness probe. Copy those binaries and CA roots into a scratch image running as UID/GID 65532. No shell, compiler, or package manager is required at runtime. The probe checks `/readyz` on the container loopback address, has a bounded timeout, refuses redirects, and fails on a non-200 response. Migrations remain explicit and separate from API startup.

Use a disposable Compose rehearsal with PostgreSQL 16 and synthetic credentials. Persist database and private upload bytes in separate named volumes; retain the upload directory's non-root ownership and 0700 permissions. API/frontend filesystems are read-only, capabilities are dropped, and privilege escalation is disabled. Publish only the frontend port on host loopback. Browser owner sign-in requires real TLS and is outside this HTTP rehearsal: production authentication continues requiring HTTPS and secure cookies. Do not loosen development authentication's loopback restriction for containers.

Add a credential-free CI job that builds images, explicitly migrates a disposable database, waits for readiness, verifies SSR/static assets and same-origin API routing/authorization, then restarts the services and repeats the checks with retained volumes. Cleanup is restricted to the unique Compose project for that CI run. This job neither publishes images nor deploys servers.

## Consequences

The two applications can be packaged independently. Rehearsal credentials and the combined local database role must never be promoted to staging/production. The local upload adapter is retained for packaging validation; production private COS, separate database roles, registry connectivity, TLS, resource sizing, backups/restoration, release manifests, and deployment/rollback workflows still need implementation and verification. A healthy frontend process alone does not prove database readiness; the stack waits for API readiness too. Container health status does not itself restart an unhealthy process or replace monitoring.
