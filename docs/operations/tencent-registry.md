# Tencent registry setup

Status: TCR Personal hostname `ccr.ccs.tencentyun.com` and namespace `isaber` confirmed by the owner's login command and namespace reply. JD login succeeded; the credential directory/file are root-owned with modes 0700/0600. Both servers reach the registry over HTTPS and receive the expected unauthenticated 401. Repository Actions secret names are configured. JD diagnostic push/remote-config verification succeeded and anonymous fixture access was denied. The first GitHub publishing run timed out; actual application publishing and digest pulls remain unverified. The earlier paid Beijing Enterprise proposal is superseded; no paid TCR instance was created by the agent.

## Proposed configuration

| Setting | Proposal |
| --- | --- |
| Region | Personal mainland service is Guangzhou; verify the actual console selection |
| Service | TCR Personal, free within quotas, subject to account availability and console limits |
| Registry hostname | Copy from console; the documented Personal hostname is `ccr.ccs.tencentyun.com` |
| Namespace | `isaber`, supplied by the owner; use private repositories |
| Application repositories | `frontend`, `backend` |
| Runtime dependency repository | `postgres`, for a verified copy of the approved PostgreSQL image |
| Image references | Deploy digests; record source commits and migration version in a release manifest |
| Promotion | Use the same application digests for JD staging and Tencent production |

The free Personal service has quota limits and no SLA. Its mainland deployment is Guangzhou; this does not require moving the production server from Beijing. Namespace names are globally unique. Confirm actual availability and limits in the console. See [Personal setup](https://cloud.tencent.com/document/product/1141/63910) and [billing](https://cloud.tencent.com/document/product/1141/40540). Enterprise remains an optional paid alternative; its dedicated instance, associated COS billing, and network policies are different from Personal.

## Account step

In the [TCR console](https://console.cloud.tencent.com/tcr), use the registry already obtained. Confirm the edition, hostname, region, and namespace and provide only those details for the workflow configuration. For Personal, initialize the registry login password if not already done, create the namespace, and create private `frontend`, `backend`, and `postgres` repositories. Do not change an existing Enterprise instance's network policy based on Personal instructions.

For an interactive host login, run `ssh jdcloud`, then use the console's `docker login` command and enter the registry password at its hidden prompt. Use the registry login credential, not a cloud API SecretKey. This logs Docker into the registry; it does not deploy the blog. Do not paste a command containing a literal password into a shell. CI will use `--password-stdin` with a GitHub Actions secret once the publishing workflow is prepared.

Passwords, cloud API keys, and registry tokens belong in the appropriate secret store, not chat or Git. The publishing workflow at `.github/workflows/publish.yml` was merged in PR #12 and is active on the default branch. JD login credentials are not copied to GitHub by this work.

Current tooling has SSH access to the servers; an authenticated Tencent account/API session has not been established. Account provisioning therefore remains an owner console step until an appropriate access mechanism is configured.

## Access design before publishing

Preserve private repository visibility. Review the selected edition's supported CAM/repository permissions before choosing credentials. Where supported, restrict publishing to the required repositories and give staging/production separate pull-only identities. Do not assume Personal provides the same service-account features as Enterprise.

The following network-policy requirements apply to **Enterprise**, not as a prerequisite for Personal setup. Enterprise access is denied by default until network policies are configured. JD Cloud needs an approved public access path; Tencent may use a verified VPC connection. The GitHub publishing job also needs an allowed egress path. Choose a controlled runner egress address or an explicitly scoped temporary CI allowlist mechanism before enabling publishing. Do not assume an ordinary hosted runner has a permanent address, or silently allow every source. See [TCR public access controls](https://cloud.tencent.com/document/product/1141/41837).

## Acceptance before deployment

1. Verify the actual registry endpoint from JD, Tencent, and the selected GitHub runner path.
2. Push a test image from trusted CI; pull its digest on both hosts and measure transfer time. Run the fixture without host mounts, published ports, or elevated privileges.
3. Verify that host credentials cannot push and CI credentials cannot administer unrelated repositories.
4. Verify the approved PostgreSQL image's origin/digest and delivery through this registry.
5. Add publishing after successful main CI, capture application digests in a release manifest, and prepare staging delivery separately.

These checks are pending. JD's successful GHCR fixture test proves Docker works; it does not validate this proposed TCR instance or its performance.


## Current release preparation and publication

The direct GitHub-to-TCR data path failed in both live attempts despite successful registry authentication. The default workflow is now **Prepare release images**: GitHub retains a verified bundle and the existing workspace relays it to JD for digest-preserving publication. See the [image relay runbook](image-relay.md) for the exact operator command and acceptance steps.

The operator command requires a successful trusted main preparation run and matching successful source CI. It uses the workspace's existing GitHub login and SSH alias and JD's existing registry credential file. It does not copy credential values between machines, install a server runner, or deploy the blog. A green preparation run means a bundle is ready; completed publication is represented only by the relay's verified `release.json`.

Keep the Personal repositories `isaber/frontend`, `isaber/backend`, and `isaber/postgres` private. Repository secrets `TCR_USERNAME` and `TCR_PASSWORD` are still configured for optional direct-route tests, but are not needed by the default operator relay. Do not reset working credentials to address this upload timeout. The registry login file on JD is root-owned with mode 0600 under a 0700 directory; that restricts access but does not encrypt the file.

Direct mode can be explicitly enabled with repository variable `TCR_PUBLISH_TRANSPORT=direct`; it is not the default. Its publishing diagnostics have flushed phases, bounded commands, and safe error categories. Actual direct-route recovery is unverified.

The first actual GitHub bundle-to-registry application release completed after PR #14, including exact-digest pulls and configuration checks on JD. The follow-up reduces the real bundle to 124.3 MiB, transferred and verified on JD in about 76 seconds, and verifies the reused database configuration against TCR. Fully unattended delivery and deployment remain subsequent work.

## First live-run findings and follow-up

PR #12 and all four main CI checks passed for source commit `71ba8704716856c74ed1aa82a8fa6301b831885d`. Its first [publishing run](https://github.com/IiCEight/iSaber/actions/runs/42461428153) passed the release rehearsal, then exceeded the 30-minute job limit inside publishing. No application release tags were readable from JD afterward, and no completed release manifest was retained. The old step captured child output and buffered its own progress, so its logs do not identify the blocked Docker operation. The follow-up adds a 20-second unauthenticated runner HTTPS preflight, flushed phase messages, bounded commands, and fixed safe error categories; it does not claim the GitHub network/credential path has been repaired.

A separate JD-to-TCR test successfully uploaded the previously verified public UV fixture under `isaber/frontend:registry-check-uv-0.13.7`. This is a diagnostic fixture, not a frontend release. It proves that JD's existing login can push to the repository; it does not prove GitHub's credential values or network path work. Application releases still use only the completed release manifest.

The fixture also reproduced a Docker-version compatibility issue: Docker 29's local `.Id` and `.RepoDigests` retained the original multi-platform index digest even though the push published only the available platform under a different digest. The follow-up publisher therefore derives `image_id` from the exact image configuration bytes in a `linux/amd64` Docker export, reads the pushed digest from Docker's tag-specific push receipt, and verifies that digest against the remote registry manifest. For an index, it resolves the unique linux/amd64 child and checks its config digest. It never treats local repository metadata as proof that a digest exists remotely.

The corrected helpers were exercised against the actual pushed fixture on JD's Docker 29, and local/remote config digests matched. The frontend diagnostic fixture was unreadable with an empty Docker credential configuration. CI additionally exports the real rehearsal images to exercise identity extraction on GitHub's Docker engine without registry credentials. Source config digests are stable across wrapper/index differences; operator checks must not compare Docker 29's default `.Id` to the manifest's `image_id` directly.


The first real release from PR #14 published all three images, and JD pulled their exact digests and verified the tested configurations. Version 2 bundles reuse the pinned PostgreSQL image in this registry, omitting its layers from each release. Keep that digest available; baseline upgrades require the explicit verification/pin-update procedure in the [relay runbook](image-relay.md). Optional direct JD artifact downloads remain opt-in because the measured GitHub-storage route was slower than the workspace relay.
