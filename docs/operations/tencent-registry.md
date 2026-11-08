# Tencent registry setup

Status: Proposed configuration for review on the staging milestone branch. The production server's region was read from its instance metadata: `ap-beijing`. No TCR instance, repository, access rule, or cloud identity has been created by this work. An existing instance may be reused after checking its region, permissions, and connectivity.

## Proposed configuration

| Setting | Proposal |
| --- | --- |
| Region | Beijing, matching the production server |
| Service | Tencent TCR Enterprise; select a tier after reviewing its current limits and displayed charges |
| Instance name | `isaber-iiceight-bj`, subject to availability |
| Namespace | `isaber`, private |
| Application repositories | `frontend`, `backend` |
| Runtime dependency repository | `postgres`, for a verified copy of the approved PostgreSQL image |
| Image references | Deploy digests; record source commits and migration version in a release manifest |
| Promotion | Use the same application digests for JD staging and Tencent production |

The actual registry hostname comes from the console; do not infer it from the proposed name. Tencent documents that instance region/name cannot be changed after creation. Review the configuration and billing summary before purchasing. The registry's automatically associated COS storage holds registry data; it does not implement the blog's private image-storage adapter. See the [official quick start](https://cloud.tencent.com/document/product/1141/39287).

## Account step

Open the [TCR console](https://console.cloud.tencent.com/tcr) and check for existing instances first. For a new instance, use the proposal above as the review checklist. Provide only the registry hostname, region, and namespace when ready. Passwords, cloud API keys, and registry tokens belong in the appropriate secret store, not chat or Git.

Current tooling has SSH access to the servers; an authenticated Tencent account/API session has not been established. Account provisioning therefore remains an owner console step until an appropriate access mechanism is configured.

## Access design before publishing

Give the publishing identity push/pull access only to the necessary repositories. Use separate pull-only identities for staging and production. Preserve private repository visibility.

TCR access is denied by default until network policies are configured. JD Cloud needs an approved public access path; Tencent may use a verified VPC connection. The GitHub publishing job also needs an allowed egress path. Choose a controlled runner egress address or an explicitly scoped temporary CI allowlist mechanism before enabling publishing. Do not assume an ordinary hosted runner has a permanent address, or silently allow every source. See [TCR public access controls](https://cloud.tencent.com/document/product/1141/41837).

## Acceptance before deployment

1. Verify the actual registry endpoint from JD, Tencent, and the selected GitHub runner path.
2. Push a test image from trusted CI; pull its digest on both hosts and measure transfer time. Run the fixture without host mounts, published ports, or elevated privileges.
3. Verify that host credentials cannot push and CI credentials cannot administer unrelated repositories.
4. Verify the approved PostgreSQL image's origin/digest and delivery through this registry.
5. Add publishing after successful main CI, capture application digests in a release manifest, and prepare staging delivery separately.

These checks are pending. JD's successful GHCR fixture test proves Docker works; it does not validate this proposed TCR instance or its performance.
