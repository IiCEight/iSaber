# Tencent registry setup

Status: TCR Personal hostname `ccr.ccs.tencentyun.com` and namespace `isaber` confirmed by the owner's login command and namespace reply. JD login succeeded; the credential directory/file are root-owned with modes 0700/0600. Both servers reach the registry over HTTPS and receive the expected unauthenticated 401. Repository visibility, authenticated image pushes/pulls, and GitHub secrets still need verification. The earlier paid Beijing Enterprise proposal is superseded; no paid TCR instance was created by the agent.

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

Passwords, cloud API keys, and registry tokens belong in the appropriate secret store, not chat or Git. The publishing workflow is now prepared locally at `.github/workflows/publish.yml`; it becomes active only after merge to the default branch. JD login credentials are not copied to GitHub by this work.

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


## GitHub publishing setup

Before merging this milestone, confirm **private** repositories `isaber/frontend`, `isaber/backend`, and `isaber/postgres` in the Personal registry. The workflow must not be used to auto-create repositories with unreviewed default visibility.

Open [repository Actions secrets](https://github.com/IiCEight/iSaber/settings/secrets/actions) and add:

| Secret | Value |
| --- | --- |
| `TCR_USERNAME` | Username from the successful console-provided Docker login command |
| `TCR_PASSWORD` | Registry login password; not a Tencent cloud API SecretKey |

Enter these directly in GitHub. Do not paste them into chat or a committed environment file. Docker login on JD does not configure these secrets. Its existing root-only credential file remains unencrypted; permissions restrict access, but do not provide encryption.

The prepared workflow runs after successful **push-to-main CI**, using the exact commit from that run. PR/fork checks and failed/manual CI runs do not publish. It builds frontend/backend images on GitHub, pulls the official PostgreSQL 16 image, and tests that exact set through the isolated migration/readiness/SSR/proxy/restart rehearsal before publishing. Registry credentials are exposed only to validation/publishing steps, not the build or test steps, and publishing uses an ephemeral Docker configuration removed on exit.

Each image receives a `sha-COMMIT` tag under `ccr.ccs.tencentyun.com/isaber/`. The publisher verifies the local image identities and repository-bound push digests before writing `release.json`. The manifest records the source commit, prior successful CI run, contract hash, migration version, application/database image IDs and registry digests, and PostgreSQL's resolved official upstream reference. Use manifest digests for deployment: tags may change on a rebuild/retry.

After merging the combined milestone PR:

1. Observe the normal main CI run, then **Publish images** in Actions.
2. Open its steps to check the release rehearsal and publishing outcome.
3. Download its `release-COMMIT-ATTEMPT` artifact and inspect `release.json`.
4. Verify the actual published images from both servers before preparing deployment.

A failed push may leave some images in the registry, but it does not produce a completed manifest or trigger deployment. Rerun the publishing workflow's jobs after fixing the cause. No server deploys automatically in this milestone. Artifacts are retained for 30 days; durable production release/rollback manifest retention remains deployment work.

Local validation covers six publisher safeguards (credentials, source validation, digest/identity binding, and failure handling) plus workflow lint. Actual GitHub publishing, private repository settings, and cross-cloud pulls are pending owner secret setup and the merged workflow run.
