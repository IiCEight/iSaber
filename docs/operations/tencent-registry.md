# Tencent registry setup

Status: The owner reports obtaining a registry; its edition, hostname, region, and namespace are awaiting confirmation. TCR Personal is the recommended starting option after reviewing the free tier. The earlier Beijing Enterprise proposal is superseded as the default; no paid instance was created by the agent. Production remains in Beijing (`ap-beijing`).

## Proposed configuration

| Setting | Proposal |
| --- | --- |
| Region | Personal mainland service is Guangzhou; verify the actual console selection |
| Service | TCR Personal, free within quotas, subject to account availability and console limits |
| Registry hostname | Copy from console; the documented Personal hostname is `ccr.ccs.tencentyun.com` |
| Namespace | Owner-selected globally available namespace, such as `isaber-iiceight`; create private repositories |
| Application repositories | `frontend`, `backend` |
| Runtime dependency repository | `postgres`, for a verified copy of the approved PostgreSQL image |
| Image references | Deploy digests; record source commits and migration version in a release manifest |
| Promotion | Use the same application digests for JD staging and Tencent production |

The free Personal service has quota limits and no SLA. Its mainland deployment is Guangzhou; this does not require moving the production server from Beijing. Namespace names are globally unique. Confirm actual availability and limits in the console. See [Personal setup](https://cloud.tencent.com/document/product/1141/63910) and [billing](https://cloud.tencent.com/document/product/1141/40540). Enterprise remains an optional paid alternative; its dedicated instance, associated COS billing, and network policies are different from Personal.

## Account step

In the [TCR console](https://console.cloud.tencent.com/tcr), use the registry already obtained. Confirm the edition, hostname, region, and namespace and provide only those details for the workflow configuration. For Personal, initialize the registry login password if not already done, create the namespace, and create private `frontend`, `backend`, and `postgres` repositories. Do not change an existing Enterprise instance's network policy based on Personal instructions.

For an interactive host login, run `ssh jdcloud`, then use the console's `docker login` command and enter the registry password at its hidden prompt. Use the registry login credential, not a cloud API SecretKey. This logs Docker into the registry; it does not deploy the blog. Do not paste a command containing a literal password into a shell. CI will use `--password-stdin` with a GitHub Actions secret once the publishing workflow is prepared.

Passwords, cloud API keys, and registry tokens belong in the appropriate secret store, not chat or Git. The next implementation needs the non-secret registry details first; no publishing workflow is enabled yet.

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
