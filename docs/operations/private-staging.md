# Private HTTPS staging on JD Cloud

Status: The owner selected private HTTPS through SSH. The verified `8cca5a4310627f9585bef4c845db278007729754` release is deployed on JD in the dedicated `isaber-staging` Compose project. Configuration/tooling is in the staging milestone PR. This is staging for synthetic content; automated GitHub deployment and production are not enabled.

## Open the site

From your local Linux terminal, keep this connection running:

```bash
ssh -N -o ExitOnForwardFailure=yes -L 127.0.0.1:3443:127.0.0.1:3443 jdcloud
```

The site is **https://localhost:3443**, and its owner dashboard is **https://localhost:3443/admin**. Use that exact hostname and port: authentication checks the configured origin. This connection forwards a local loopback port to JD's loopback HTTPS listener. Closing it removes local access; the staging containers continue running.

Before browser sign-in, import the **public** staging CA certificate into your browser's trusted certificate authorities. It is available locally at `infra/releases/staging/staging-root.crt` (ignored by Git), or copy it again:

```bash
scp jdcloud:/opt/isaber-staging/staging-root.crt ./staging-root.crt
openssl x509 -in ./staging-root.crt -noout -fingerprint -sha256
```

Observed public certificate fingerprint:

```text
0F:84:20:05:2E:86:52:59:06:46:15:93:63:9F:40:E3:AB:E2:4F:FE:12:F7:F2:FF:B8:29:88:A1:DF:C4:BC:E4
```

The browser's certificate-authority settings have an Import action; select this `.crt` and enable trust for websites. Browser/OS trust stores can differ, including Windows browsers with a WSL terminal. No local/global trust configuration was changed automatically. The CA private key remains in JD's dedicated certificate volume. [Caddy local HTTPS](https://caddyserver.com/docs/automatic-https#local-https) requires clients to trust its CA.

With the tunnel open, this checks HTTPS without changing any trust store:

```bash
curl --cacert infra/releases/staging/staging-root.crt https://localhost:3443/healthz
```

Expected response: `{"status":"ok"}`. Do not bypass certificate verification for owner acceptance.

## Create your staging owner

No owner remains after acceptance. Run this command in your terminal; the password prompt is hidden and asks for confirmation:

```bash
ssh -t jdcloud 'docker compose --project-name isaber-staging --env-file /opt/isaber-staging/secrets.env --env-file /opt/isaber-staging/release.env --file /opt/isaber-staging/config/compose.yml run --rm --no-deps owner setup --username owner'
```

Choose a unique staging passphrase with at least 15 characters. Enter it in the terminal, not chat or an argument. Then sign in at `https://localhost:3443/admin`. The command refuses to overwrite an existing owner. For intentional recovery, use the same operator command with `recover` instead of `setup`; recovery revokes old sessions. See the [owner guide](owner-account.md).

## What is running

| Component | Access and persistence | Limit |
| --- | --- | --- |
| Caddy 2.11.9 | Host `127.0.0.1:3443`; certificate/config volumes | 128 MiB / 0.25 CPU |
| Next.js frontend | Container networks only | 512 MiB / 0.75 CPU |
| Go API | Container networks; dedicated uploads volume | 256 MiB / 0.5 CPU |
| PostgreSQL 16 | Internal database network; dedicated database volume | 384 MiB / 0.5 CPU |

Docker creates the project's bridge networks and published-port forwarding. Only the HTTPS port is host-published, on IPv4 loopback. No host port is published for PostgreSQL, Go, or Next.js. Existing host ports 80/443 remain owned by the relay. Existing SSH, relay, Kubernetes, Tailscale, Tencent workloads, and their configuration were not replaced. Docker/SSH/relay remained active after acceptance; this is not a full functional audit of those other services.

The API uses a dedicated runtime database role. It cannot perform DDL, update migration bookkeeping, create owners, or update their password hashes. Login's row locking needs an UPDATE privilege on a column; only the constrained `singleton` column is granted, whose check/not-null constraints keep it true. [PostgreSQL SELECT privileges](https://www.postgresql.org/docs/16/sql-select.html) describe the row-lock requirement. Migration and owner commands use the separate operator connection.

Server-generated database/CSRF secrets are under `/opt/isaber-staging/secrets.env`, mode 0600 inside a 0700 directory. They are stable across restart and are never copied to GitHub or the workspace. Current binaries receive secrets through their environment; root/Docker operators can inspect them. Avoid printing interpolated Compose configuration or container environments. Secret-manager/file-based application configuration is future hardening.

All containers have bounded Docker log rotation. Application/proxy containers use a read-only root filesystem and dropped capabilities. PostgreSQL receives the capabilities needed for its official entrypoint's volume ownership and user switch. The stack's declared memory limits total 1,280 MiB, leaving capacity for existing host services; continue monitoring the 2 GiB host.

## Images and first deployment

Application/database references come from the verified release manifest in TCR. The optimized release relay transferred **124.3 MiB in 75.8 seconds**, reused the existing database digest, and verified all three remote configurations.

The HTTPS proxy is a one-time offline bootstrap dependency. Its official linux/amd64 upstream manifest and config digests are recorded in [`proxy-image.json`](../../infra/staging/proxy-image.json). Skopeo verified the official image before Docker-archive transfer through SSH. On JD, the helper hashes the exported config and resolves Docker 29's separate immutable engine ID; Compose runs that ID with pulling disabled. It never relies on a mutable proxy tag for execution. No new registry repository or host web-server package was needed. Keep the loaded proxy image; do not blindly prune it. A proxy upgrade needs another verified offline delivery and reviewed pin update.

Configuration is installed at `/opt/isaber-staging/config/`, with the shared image-identity helper at `/opt/isaber-staging/publish_images.py`. Initial operations were:

```bash
python3 /opt/isaber-staging/config/staging.py init
python3 /opt/isaber-staging/config/staging.py deploy --release /opt/isaber-staging/candidate-release.json
```

These commands run **on JD**, not the workstation. `init` preserves existing secrets and refuses to regenerate them over an existing database volume. `deploy` validates the release, pulls exact images, explicitly migrates, grants runtime permissions, starts the stack, verifies its CA/HTTPS routes, and records `current-release.json` only after success. Initial failure stops the application/proxy while retaining data/secrets for diagnosis. The initial deployer refuses to update an existing successful staging instance; coordinated backup and reviewed promotion/rollback are required first.

## Operator checks and stopping

```bash
ssh jdcloud 'python3 /opt/isaber-staging/config/staging.py status'
ssh jdcloud 'python3 /opt/isaber-staging/config/staging.py verify'
```

To stop only this project, preserving all data:

```bash
ssh jdcloud 'docker compose --project-name isaber-staging --env-file /opt/isaber-staging/secrets.env --env-file /opt/isaber-staging/release.env --file /opt/isaber-staging/config/compose.yml stop'
```

To restart it:

```bash
ssh jdcloud 'docker compose --project-name isaber-staging --env-file /opt/isaber-staging/secrets.env --env-file /opt/isaber-staging/release.env --file /opt/isaber-staging/config/compose.yml up -d --wait postgres api frontend edge'
```

Do not use `down --volumes`, global container/volume pruning, or broad service restarts. Keep database, uploads, secrets, CA storage, and release metadata together for recovery. Node/Go toolchains and application package installations are not needed on JD.

## Acceptance and next milestone

The initial cloud acceptance used a random temporary owner/password held in process memory, with no credential values printed. It checked runtime/operator privilege separation, CA-verified HTTPS login, secure `__Host-` cookie attributes, authenticated dashboard SSR, unauthorized origin rejection, missing-CSRF rejection, session/database/CA persistence across restart, and logout. Its owner and sessions were removed afterward. A temporary local SSH tunnel also passed a CA-verified HTTPS health check and was closed after verification. The acceptance command intentionally refuses to operate over an existing owner account.

Local tests cover manifest boundaries, private secret generation/idempotency, refusal to replace credentials over an existing database, and refusal to redeploy a data-bearing instance. Real container/migration checks remain in CI. Interactive browser acceptance is the next owner exercise.

Coordinated database/media backup and disposable restore verification, schema-aware promotion/rollback, a restricted deployment identity, and unattended GitHub delivery remain the next operational milestone. Until recovery is verified, use synthetic staging content only. Private COS and public production hosting remain separate work.
