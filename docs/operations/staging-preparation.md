# Guided staging preparation

Status: Server inventory, package review, and JD Docker/Compose installation completed on 2026-11-08 under the owner's instruction to execute directly. Container packaging is merged in PR #11; all four GitHub checks passed on the PR and main. The owner confirmed both hosts are in mainland China; use domestic package delivery and plan a mainland image registry. Blog deployment has not started. Keep this preparation on one milestone branch while working through the exercises.

## 1. Observed constraints

| Check | JD Cloud: proposed staging | Tencent Cloud: proposed production |
| --- | --- | --- |
| OS / architecture | Ubuntu 24.04.2, x86_64 | Ubuntu 24.04.4, x86_64 |
| Available RAM at inspection | About 1.5 GiB, no swap | About 2 GiB, swap configured |
| Free root disk | About 25 GB | About 22 GB |
| Existing workloads | Relay owns public ports 80/443; a Kubernetes agent is present and activating | Several active services, Kubernetes, PostgreSQL, Docker containers, and private HTTPS listeners |
| Docker / Compose | Installed after review: Docker 29.1.3 / Compose 2.40.3 | Docker client/server 29.1.3 work through existing sudo; Compose plugin absent |
| Host UFW | Inactive | Inactive |
| GHCR | Full public fixture pull and isolated non-root run passed, but about 19 MB compressed took roughly ten minutes | Authentication/manifest/one-byte layer sample passed; full pull not tested |
| Direct Docker Hub API | Timed out | Timed out |
| Docker package host | TLS connection reset during the probe | Not tested |
| Ubuntu package archive | Metadata refreshed successfully; reviewed packages installed from configured signed Ubuntu repositories | Not tested |

These are observations, not resource reservations or an external firewall audit. Cloud security-group rules were not inspected. An inactive UFW does not establish that every port is externally reachable. JD Cloud's installed Tailscale client reports `NeedsLogin`; its presence does not provide a working private deployment path. The Kubernetes agent's `activating` state does not establish a healthy cluster.

Existing workloads must remain in place. Do not replace the relay on JD ports 80/443, reuse Tencent's existing PostgreSQL database, change Docker-group membership, or uninstall runtime packages as part of a generic installation recipe. The installation saved a root-only network/service baseline under `/root/isaber-preflight.*` before starting Docker. Docker adds its own bridge and firewall rules; this must not be described as an unchanged firewall. SSH and the relay remained active with the same processes, and no existing runtime packages were removed.

## 2. Staging access proposal

Start with private access through the existing SSH connection, keeping the blog's host port on loopback. This is the recommended direction, pending the owner's access preference. A public subdomain needs a separately agreed routing/TLS arrangement because JD's web ports are occupied.

The existing [container rehearsal](containers.md) can first verify packaging and connectivity on an unused loopback port. It remains an HTTP rehearsal with synthetic configuration; it does not support browser owner sign-in. Before owner publishing acceptance, configure a reviewed HTTPS endpoint and matching `SITE_ORIGIN`, retaining secure cookies and exact-Origin/CSRF checks. An SSH tunnel alone does not turn HTTP into an HTTPS browser origin. Do not weaken authentication to accommodate container networking.

Docker installation introduced its normal system services and networking. Existing DNS, relay, Tailscale, and Kubernetes configuration were not edited. TLS and deployment connectivity remain separate work.

## 3. Completed package review and installation

The following review commands were executed directly **on JD Cloud** after the owner delegated the exercise. They are recorded for learning; do not repeat installation over an existing setup:

```bash
ssh jdcloud
apt-get update
apt-cache policy docker.io docker-compose-v2 containerd runc
apt-get --simulate install docker.io docker-compose-v2
```

The inspected JD SSH account is root, so these commands do not need `sudo`. `apt-get update` refreshes package metadata; it does not install Docker or upgrade applications. `--simulate` shows the proposed package changes without performing them. Run each command separately and review its output. If metadata refresh reports errors, retain the error text and diagnose it before proceeding; a partial update is not a verified package source.

Ubuntu provides `docker.io` and the `docker-compose-v2` package. This route was used because the Ubuntu archive was reachable while Docker's own package host failed during inspection. Actual candidates were verified from the configured Ubuntu repositories before installation. Do not add an arbitrary registry/package mirror or pipe an installer into a root shell to bypass the failure.

The reviewed plan added eight packages, with zero upgrades and zero removals. Installed versions:

| Package | Version |
| --- | --- |
| docker.io | 29.1.3-0ubuntu3~24.04.2 |
| docker-compose-v2 | 2.40.3+ds1-0ubuntu1~24.04.1 |
| containerd | 2.2.1-0ubuntu1~24.04.4 |
| runc | 1.3.4-0ubuntu1~24.04.1 |

After saving the network baseline and confirming no pre-existing Docker daemon configuration, installation used:

```bash
DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=l apt-get --yes --no-remove install docker.io=29.1.3-0ubuntu3~24.04.2 docker-compose-v2=2.40.3+ds1-0ubuntu1~24.04.1
```

`--no-remove` prevents package removals. `NEEDRESTART_MODE=l` reports rather than automatically restarts unrelated services; package setup still starts the new Docker/containerd services. No pre-existing Kubernetes/runtime packages were removed. The installer reported no services needing restart. Docker client/server and Compose reported the expected versions; the storage driver is overlayfs and cgroups v2 are active. Docker/containerd, SSH, and the relay were active afterward. The Kubernetes agent retained its prior `activating/start` state; its existing issue was not repaired or reconfigured by this work. No users were added to the Docker group. A fresh SSH connection succeeded and the existing relay returned HTTP 302 on loopback after installation. These checks do not establish external relay or Kubernetes end-to-end health.

## 4. Registry and delivery plan

A registry hosted in mainland China is now the preferred direction for application delivery, following the owner's clarification that both servers are in China. TCR Personal is the recommended starting point after reviewing its free tier; the owner reports obtaining a registry and its details are pending. See the [registry setup guide](tencent-registry.md). Personal mainland service is Guangzhou; production remains in Beijing (`ap-beijing`). Its actual endpoint, region, access policies, and throughput must be verified before selection. GHCR is retained as a diagnostic/source option, not the assumed deployment registry. The network probe obtained an anonymous token for a public fixture image, resolved its Linux/amd64 manifest, and read one byte from a layer with HTTP 206. The subsequent JD full pull also succeeded for `ghcr.io/astral-sh/uv@sha256:96f40d3904aad8d0bd010539001d36c8e2188b981e1334471a90c4a1d93bb490`. Its roughly 19 MB compressed layers took about ten minutes, with observed transfer progress around 30–40 KB/s. The digest-pinned image then ran `/uv --version` as UID/GID 65532 with networking disabled, a read-only filesystem, dropped capabilities, and no host mounts; it reported uv 0.13.7 and the temporary container was removed. This verifies one fixture and runtime execution, not acceptable throughput for the larger application/database images. Those actual images and both servers still need registry-specific acceptance.

Build frontend/backend images on GitHub, then publish immutable digests to the selected mainland registry after trusted CI succeeds. Test the GitHub-runner-to-registry push path as well as pulls from both clouds; registry location alone does not prove end-to-end performance. Cloud hosts should pull the completed images. Docker Hub failures mean a cloud deployment cannot yet assume that `postgres:16` is pullable. Provide a verified database-image delivery path as part of the release milestone; do not silently substitute an unverified third-party mirror. Decide package visibility and pull authentication before configuring publishing. Public source visibility does not automatically make a new container package public.

The proposed delivery path is `GitHub Actions → owner-controlled mainland registry → JD staging / Tencent production`. Mirror approved third-party runtime images, including PostgreSQL, into that registry by verified digest as needed. Keep CI builds on GitHub using the existing locked dependencies; cloud servers pull finished images and do not need Node/Go package installation for deployment.

The successful Docker package installation already used JD Cloud's domestic Ubuntu mirror, `mirrors.jdcloudcs.com`; 88.5 MB of packages downloaded in about eight seconds. Keep the existing working package source. OS APT mirrors, npm/Go dependency mirrors, and container registries solve different transfers; do not change them as one global setting.

Tencent's documented overseas-image acceleration is restricted to supported Tencent users and Tencent VPC access; do not assume that it is a general public accelerator usable from JD Cloud. A TCR repository with an explicitly reviewed access configuration is a different service path. See [Tencent acceleration restrictions](https://cloud.tencent.com/document/product/457/51237) and [TCR network setup](https://cloud.tencent.com/document/product/1141/39287). New registry provisioning and broad network access changes are not part of the completed host installation.

The rehearsal's synthetic database credentials and CSRF key remain local/CI fixtures. Real staging needs separate runtime secrets, a staging owner, persistent media/database storage, resource limits, and coordinated backup/restore verification. Production private COS remains unimplemented. Registry endpoints being reachable does not settle GitHub-to-server SSH connectivity or host-key verification.

## 5. Remaining guided steps

1. Completed: review and install Docker/Compose on JD, then verify client/server versions and existing SSH/relay processes.
2. Verify a full pull, then benchmark the selected mainland registry from both clouds and a CI push; configure repository permissions and the approved database image source.
3. Review staging HTTPS access, deployment identity/connectivity, and SSH host-key verification. Keep private credentials in server/GitHub secret stores.
4. Prepare staging-specific Compose configuration, persistent data, resource/log limits, and backup/restore commands.
5. Add trusted image publishing and release manifests, then staging delivery with explicit migrations, health checks, acceptance, and rollback rehearsal.

Do not open a separate PR for each exercise. Keep focused commits on this milestone branch, then review one coherent preparation/delivery PR when its implemented scope and validation evidence are ready.

## References

- [Ubuntu docker-compose-v2 package](https://packages.ubuntu.com/noble/docker-compose-v2): distro packaging and Docker dependency; installed candidates depend on enabled updates/security repositories.
- [Docker's Ubuntu installation guide](https://docs.docker.com/engine/install/ubuntu/): the alternative Docker CE path, prerequisites, and runtime package conflicts. Its uninstall instructions must not be applied blindly to these shared hosts.
