# Single-host delivery

Owner scope change, November 11, 2026: one Tencent blog deployment, no separate staging pipeline, no scheduled backup/offsite/retention system. Article data and uploads remain in persistent volumes. The old implementation is available in Git history.

## Release path

`PR checks → main checks/build/test → GHCR → NJU → Tencent Docker Compose`

The single [CI workflow](../../.github/workflows/ci.yml) keeps the API contract, frontend, backend, and Containers checks. The Containers job builds application images once, tests them against PostgreSQL with the production runtime-role grants, and publishes those same local images after successful main checks. It does not export/repackage images or upload release bundles. PRs do not publish or deploy.

GHCR packages are `ghcr.io/iiceight/isaber-frontend` and `ghcr.io/iiceight/isaber-backend`. Confirm both packages are public (the owner reports this is already set); public image layers contain application code, while credentials, articles, and uploads remain outside the Docker build contexts. CI uses its built-in `GITHUB_TOKEN`; the host needs no registry token. Full-image mirror checks passed, with initial short-limit timeouts followed by successful retries; cold pulls require allowance. The server pulls `ghcr.nju.edu.cn/iiceight/isaber-frontend@sha256:…` and the corresponding backend digest. See the [mirror evaluation](ghcr-mirror-evaluation.md).

Deployment sends two digest lines through a dedicated SSH key. The host's [small deployer](../../infra/deploy.sh) validates them, pulls images through NJU, starts the application services, and checks CA-verified HTTPS. `previous.env` retains the previous application references. Failed activation attempts restore the preceding images; rollback never rewinds the database or uploads. No release performs database migrations, backup transfers, registry mirroring, or volume cleanup.

## Current installation — November 11, 2026

PR #26 is merged. The blog runs at **https://101.42.13.215**; owner sign-in is at [/admin](https://101.42.13.215/admin). The owner chose direct IP access and preservation of the JD account/article. Tencent has one owner, one article, and zero managed images. Owner credentials, article/revision/media row fingerprints and all upload-file checksums matched JD before acceptance; no password reset or article rewrite was performed.

Caddy 2.11.9 uses Let's Encrypt's `shortlived` ACME profile for the public IP and renews the certificate automatically. `default_sni` selects this certificate for clients that omit SNI when connecting to an IP. The initial certificate is valid through November 18, 2026; HTTP redirects to HTTPS. The owner opened public TCP 443; port 80 is also available for HTTP challenges. The certificate and both public HTTP/HTTPS were checked from external networks. See [Let's Encrypt IP certificates](https://letsencrypt.org/2026/01/15/6day-and-ip-general-availability) and [Caddy TLS profiles](https://caddyserver.com/docs/caddyfile/directives/tls).

The Compose plugin was installed from Tencent's Ubuntu mirror with no package upgrades/removals and no Docker restart. The blog binds `10.2.0.6:80/443`, preserving Tailscale's separate HTTPS listener, Fieldnotes/private Caddy, host PostgreSQL, RSSHub, k3s, Mihomo and feed services. The four blog containers used about 228 MiB combined at inspection, under their configured limits. Database, uploads, certificates and proxy configuration use project-scoped persistent volumes; no database port is published.

GitHub's `production` environment permits only the `main` branch. It holds a dedicated forced-command SSH key and the pinned host key. Arbitrary commands through that key were rejected. `BLOG_DEPLOY_ENABLED=true` with explicit owner approval. The initial installation was verified through the operator connection; automatic CI-to-Tencent delivery then passed on the PR #27 main merge.

JD's four old staging containers are stopped; original volumes, account/article data, keys and backup archives remain. Its backup/retention timers are disabled, Tencent retention remains disabled, and `STAGING_CD_ENABLED=false`. No scheduled backup transfer or separate staging delivery is part of the new installation.

Public HTTPS, process/database readiness, Home/Archive/Search, static assets, existing article visibility, anonymous owner-route rejection and cross-origin rejection passed. The real deployer also passed a same-release update with CA-verified HTTPS. Interactive owner/browser acceptance uses the preserved JD credentials.

## First automatic production release

The owner explicitly enabled ongoing automatic delivery on November 11. [Main run 43019383308](https://github.com/IiCEight/iSaber/actions/runs/43019383308), from PR #27 merge `0d340ee3386211d3782915e1cdfcbe6fcdc61dc6`, passed all four checks and **Deploy blog**. The deploy job took 3 minutes 17 seconds. GitHub published the already-tested images, sent only their digests through the restricted SSH key, and Tencent pulled them through NJU and passed its verified HTTPS health check.

| Component | CI-published and running digest |
| --- | --- |
| Frontend | `sha256:a563814bde707f17ace12a8e35d736ec9ecc2985034751c3e1bc5fe232320194` |
| Backend | `sha256:13824e5d451ddc6f45cec9d4b13b7986323187d6a119694aa90f46ba4bddd2e9` |

The active containers and `release.env` matched both digests. `previous.env` retained the preceding release references. PostgreSQL was not restarted; one owner, one article and zero managed images remained. Public Home/Archive/Search, article-list and HTTPS health requests returned 200 afterward, and RSSHub remained running. These are observed timings for this release, not a cold-pull performance guarantee. Explicit live rollback rehearsal remains separate from the already-passing rollback failure-path tests.

## One-time Tencent setup

The first setup and JD content cutover above are complete. The steps below are a reference for a new empty host; preserve an existing installation and its credentials. Tencent is shared: preserve Fieldnotes, its host PostgreSQL, RSSHub, k3s, Mihomo, Tailscale, and the existing encrypted vault. The new Compose project is named `isaber`; its PostgreSQL has no host port and its volumes are separate from the host database. The pinned official PostgreSQL/Caddy images pull through DaoCloud; NJU’s Docker Hub endpoint returned HTTP 403 from this host. These dependency images are independent of the GHCR application route. Select the host LAN address for `BLOG_BIND_ADDR`, avoiding the existing Tailscale listeners. Do not bind blindly to all interfaces.

1. Inspect capacity/occupied ports and review installation of the Compose v2 plugin (Docker is already installed). Simulate the package change before installing; do not upgrade/remove unrelated runtime packages or restart Docker for registry/proxy configuration. Confirm `docker compose version` works.
2. Set `BLOG_DOMAIN` to the public IPv4 address or DNS name. The initial choice is `101.42.13.215`. Allow public TCP 80/443. Caddy obtains a publicly trusted short-lived certificate; API authentication uses exactly `https://BLOG_DOMAIN`. A domain can be configured later with matching DNS and origin settings.
3. Through the existing operator SSH connection, install the reviewed files:

   ```text
   /opt/isaber/compose.yml          infra/compose.yml
   /opt/isaber/Caddyfile            infra/Caddyfile
   /opt/isaber/grant-runtime.sql    infra/grant-runtime.sql
   /opt/isaber/deploy.sh            infra/deploy.sh
   /usr/local/libexec/isaber-ssh    infra/ssh-entry.sh
   ```

   `/opt/isaber` is root-owned mode 0700; `deploy.sh` is mode 0700. The SSH entry is outside that private directory, root-owned mode 0755, so the login account can execute it. Other configuration is root-owned mode 0600.

4. Create `/opt/isaber/.env` from [production.env.example](../../infra/production.env.example). Use unquoted domain/IPv4 values. Generate separate passwords with `openssl rand -hex 32` and the CSRF key with `openssl rand -base64 32`. Write values only on the server, mode 0600. Keep them stable across releases. Do not regenerate credentials over an existing database volume.
5. Create `/opt/isaber/release.env`, mode 0600, with the exact accepted image digests from CI:

   ```dotenv
   FRONTEND_IMAGE=ghcr.nju.edu.cn/iiceight/isaber-frontend@sha256:FRONTEND_DIGEST
   BACKEND_IMAGE=ghcr.nju.edu.cn/iiceight/isaber-backend@sha256:BACKEND_DIGEST
   ```

6. As the operator, enter a root shell on Tencent, then use this helper for commands scoped to the new project:

   ```bash
   cd /opt/isaber
   dc() { docker compose --project-name isaber --env-file .env --env-file release.env --file compose.yml "$@"; }
   dc config --quiet
   dc pull postgres edge api frontend
   dc up -d --wait postgres
   ```

7. Choose the data path before starting application writers:
   - **New empty blog:** run `dc run --rm --no-deps -T migrate up </dev/null`, then the grants below. Provision the owner afterward with `dc run --rm --no-deps owner setup --username owner` at a private terminal prompt.
   - **Preserve JD content:** coordinate one final stop of JD's blog writers, then transfer its logical database dump and uploads once into the new empty project's database/uploads volume. Use `pg_restore --no-owner --no-privileges --exit-on-error`; do not restore into Tencent's host PostgreSQL or any existing populated database. Preserve article/asset identifiers and permissions. Check row/media counts and migration status before starting writers. The restored owner remains; do not rerun setup over it. This is the cutover, not a recurring backup service.

   Apply the reviewed runtime grants after migration/restoration:

   ```bash
   dc exec -T postgres psql -X -v ON_ERROR_STOP=1 -U isaber_operator -d isaber < grant-runtime.sql
   dc up -d --wait api frontend edge
   ```

8. Verify public HTTPS, `/healthz`, Home/Archive/Search, owner sign-in, a private article's anonymous 404, and managed images. Verify the original content if migrated. Keep the JD instance until this acceptance passes; after cutover, stop its old blog project without removing data volumes.

## GitHub deployment setup

Generate a dedicated Ed25519 deployment key. Append its **public** key to the existing `ubuntu` account's authorized_keys without replacing existing entries:

```text
restrict,command="/usr/local/libexec/isaber-ssh" ssh-ed25519 PUBLIC_KEY isaber-blog-deploy
```

The forced command accepts only `deploy` or `rollback` and invokes the fixed root-owned deployer through the account's existing operator sudo access. It accepts no paths, executable code, arbitrary SSH commands, or forwarded connections. Keep the operator's general SSH private key out of GitHub.

Configure GitHub's `production` environment with a branch policy allowing only main:

| Setting | Value |
| --- | --- |
| Secret `BLOG_SSH_KEY` | Dedicated deployment private key |
| Variable `BLOG_SSH_HOST` | Tencent SSH hostname/IP |
| Variable `BLOG_SSH_USER` | `ubuntu` |
| Variable `BLOG_SSH_HOST_KEY` | Tencent's Ed25519 public host key, verified through the existing trusted connection |
| Repository variable `BLOG_DEPLOY_ENABLED` | `true` only after host/data/HTTPS acceptance and approval to activate ongoing releases |

As of November 11, the main ruleset requires pull requests and API contract. Frontend, Backend and Containers run and gate delivery; requiring those three before merge is still pending. An explicit live rollback rehearsal is also pending; see the [next priorities](../product/backlog.md#next-priorities).

Main runs are serialized and allowed to finish deployment/rollback. SSH verifies the pinned host key. Deployment logs stay private at `/opt/isaber/deploy.log`; GitHub receives a short success/failure message. The deployment lock prevents simultaneous operator/CI updates. A sudden host power loss or forced kill can require the manual rollback command; this is not a general disaster-recovery framework.

## Operations

Run normal inspection commands through the operator account, scoped with `dc` as above. Keep `.env`, database, uploads, and certificate volumes in place across releases.

- Status: `dc ps`.
- Application rollback: `sudo /opt/isaber/deploy.sh rollback` or `ssh -i DEPLOY_KEY ubuntu@HOST rollback` through the restricted key.
- Migration: explicitly select the reviewed candidate backend digest in `release.env`, run `dc run --rm --no-deps -T migrate up </dev/null`, then `dc exec -T postgres psql -X -v ON_ERROR_STOP=1 -U isaber_operator -d isaber < grant-runtime.sql`. Review backward compatibility before applying schema changes; switching image references cannot undo a schema migration.
- Owner recovery: use `dc run --rm --no-deps owner recover --username owner` at the operator terminal. See the [owner guide](owner-account.md).
- No automatic backup/retention job is part of this deployment. Any manual export is a separate owner operation. Do not use `down --volumes` or global pruning on the live project.

## Legacy retirement record and procedure

The November 11 cutover completed these retirement steps. Stored JD/Tencent data and archives were retained. Use the list below when reconciling an older installed copy.

1. Disable new JD deliveries with repository variable `STAGING_CD_ENABLED=false`; let active delivery/rollback finish. The replacement removes the three legacy workflows from main.
2. On JD, disable future captures/retention: `systemctl disable --now isaber-offsite.timer isaber-retention-jd.timer`. Let any already-running backup service finish its cleanup; do not kill it while writers are paused.
3. On Tencent, disable the already-disabled legacy retention schedule with `sudo systemctl disable --now isaber-retention-vault.timer`. Leave the existing vault/keys/ciphertext in place until any deletion is separately requested; these files incur no recurring transfer.
4. After content/HTTPS acceptance, stop only JD's `isaber-staging` application project, preserving its volumes. Remove unused GitHub staging/deployment/monitoring secrets and variables after confirming no old run is active.
5. Record the accepted production release and content counts. No JD/Tencent root directories, other applications, stored journals, existing backups, or shared Docker resources are deleted by this change.

References: [GitHub image publication](https://docs.github.com/en/actions/tutorials/publish-packages/publish-docker-images), [Compose pull](https://docs.docker.com/reference/cli/docker/compose/pull/), [Compose up](https://docs.docker.com/reference/cli/docker/compose/up/).
