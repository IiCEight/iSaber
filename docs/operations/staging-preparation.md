# Guided staging preparation

Status: Read-only server and network inventory completed on 2026-11-08. Container packaging is merged in PR #11; all four GitHub checks passed on the PR and main. Server installation and cloud deployment have not started. Keep this preparation on one milestone branch while working through the exercises.

## 1. Observed constraints

| Check | JD Cloud: proposed staging | Tencent Cloud: proposed production |
| --- | --- | --- |
| OS / architecture | Ubuntu 24.04.2, x86_64 | Ubuntu 24.04.4, x86_64 |
| Available RAM at inspection | About 1.5 GiB, no swap | About 2 GiB, swap configured |
| Free root disk | About 25 GB | About 22 GB |
| Existing workloads | Relay owns public ports 80/443; a Kubernetes agent is present and activating | Several active services, Kubernetes, PostgreSQL, Docker containers, and private HTTPS listeners |
| Docker / Compose | Neither command installed | Docker client/server 29.1.3 work through existing sudo; Compose plugin absent |
| Host UFW | Inactive | Inactive |
| GHCR | Authentication, public Linux/amd64 manifest, and one-byte layer sample succeeded | Same checks succeeded |
| Direct Docker Hub API | Timed out | Timed out |
| Docker package host | TLS connection reset during the probe | Not tested |
| Ubuntu package archive | Reachable; current package metadata has no Docker candidates | Not tested |

These are observations, not resource reservations or an external firewall audit. Cloud security-group rules were not inspected. An inactive UFW does not establish that every port is externally reachable. JD Cloud's installed Tailscale client reports `NeedsLogin`; its presence does not provide a working private deployment path. The Kubernetes agent's `activating` state does not establish a healthy cluster.

Existing workloads must remain in place. Do not replace the relay on JD ports 80/443, reuse Tencent's existing PostgreSQL database, change Docker-group membership, or uninstall runtime packages as part of a generic installation recipe. Docker installation may affect host networking; review the proposed packages and network plan before starting the daemon on this shared host.

## 2. Staging access proposal

Start with private access through the existing SSH connection, keeping the blog's host port on loopback. This is the recommended direction, pending the owner's access preference. A public subdomain needs a separately agreed routing/TLS arrangement because JD's web ports are occupied.

The existing [container rehearsal](containers.md) can first verify packaging and connectivity on an unused loopback port. It remains an HTTP rehearsal with synthetic configuration; it does not support browser owner sign-in. Before owner publishing acceptance, configure a reviewed HTTPS endpoint and matching `SITE_ORIGIN`, retaining secure cookies and exact-Origin/CSRF checks. An SSH tunnel alone does not turn HTTP into an HTTPS browser origin. Do not weaken authentication to accommodate container networking.

No DNS, firewall, relay, Tailscale, or Kubernetes configuration is changed by this plan. Installing Docker and configuring TLS/deployment connectivity are separate guided actions after reviewing the inventory.

## 3. Owner exercise: refresh package metadata and review the install plan

Run these commands **on JD Cloud**, starting from the local Linux terminal:

```bash
ssh jdcloud
apt-get update
apt-cache policy docker.io docker-compose-v2 containerd runc
apt-get --simulate install docker.io docker-compose-v2
```

The inspected JD SSH account is root, so these commands do not need `sudo`. `apt-get update` refreshes package metadata; it does not install Docker or upgrade applications. `--simulate` shows the proposed package changes without performing them. Run each command separately and review its output. If metadata refresh reports errors, retain the error text and diagnose it before proceeding; a partial update is not a verified package source.

Ubuntu provides `docker.io` and the `docker-compose-v2` package. This is a candidate route because the Ubuntu archive was reachable while Docker's own package host failed during inspection. Confirm actual candidates from the configured, signed Ubuntu repositories before installation. Do not add an arbitrary registry/package mirror or pipe an installer into a root shell to bypass the failure.

The package plan must have valid candidates, include Compose v2, and be reviewed for removals or changes affecting existing runtimes. Do not run the real install yet. Once the output is available, review the exact package versions and changes and prepare the installation/network verification step. Do not remove existing Kubernetes/runtime packages to satisfy a Docker CE recipe.

## 4. Registry and delivery plan

GHCR is the current candidate for application delivery. The network probe obtained an anonymous token for a public fixture image, resolved its Linux/amd64 manifest, and read one byte from a layer with HTTP 206. This exercises more than the registry's expected unauthenticated 401 response, but is not a full image pull or throughput test. Repeat with the actual application and database images before selecting it for deployment.

Build frontend/backend images on GitHub, then publish immutable digests after trusted CI succeeds. Cloud hosts should pull the completed images. Docker Hub failures mean a cloud deployment cannot yet assume that `postgres:16` is pullable. Provide a verified database-image delivery path as part of the release milestone; do not silently substitute an unverified third-party mirror. Decide package visibility and pull authentication before configuring publishing. Public source visibility does not automatically make a new container package public.

The rehearsal's synthetic database credentials and CSRF key remain local/CI fixtures. Real staging needs separate runtime secrets, a staging owner, persistent media/database storage, resource limits, and coordinated backup/restore verification. Production private COS remains unimplemented. Registry endpoints being reachable does not settle GitHub-to-server SSH connectivity or host-key verification.

## 5. Remaining guided steps

1. Review the package simulation, install the agreed Docker/Compose packages on JD, and verify existing service health and network access afterward.
2. Verify a real image pull and choose registry visibility/authentication and the database image source.
3. Review staging HTTPS access, deployment identity/connectivity, and SSH host-key verification. Keep private credentials in server/GitHub secret stores.
4. Prepare staging-specific Compose configuration, persistent data, resource/log limits, and backup/restore commands.
5. Add trusted image publishing and release manifests, then staging delivery with explicit migrations, health checks, acceptance, and rollback rehearsal.

Do not open a separate PR for each exercise. Keep focused commits on this milestone branch, then review one coherent preparation/delivery PR when its implemented scope and validation evidence are ready.

## References

- [Ubuntu docker-compose-v2 package](https://packages.ubuntu.com/noble/docker-compose-v2): distro packaging and Docker dependency; installed candidates depend on enabled updates/security repositories.
- [Docker's Ubuntu installation guide](https://docs.docker.com/engine/install/ubuntu/): the alternative Docker CE path, prerequisites, and runtime package conflicts. Its uninstall instructions must not be applied blindly to these shared hosts.
