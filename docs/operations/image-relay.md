# Recover image delivery through the existing workspace

The direct GitHub-to-TCR frontend upload timed out after successful authentication. The default workflow now prepares a tested image bundle, and an operator command relays it through the workspace to JD for publication. See [ADR 0012](../architecture/decisions/0012-image-bundle-relay.md).

## What is automatic

After successful push-to-main CI, **Prepare release images** builds the frontend/backend images, pulls official PostgreSQL 16, and runs the existing container acceptance flow. It exports compressed image blobs, configuration identities, source/CI metadata, and the reviewed relay tooling into an artifact named `bundle-COMMIT-ATTEMPT`.

The bundle is retained for **one day**. GitHub artifact storage allowance applies; this does not require another registry subscription. No password, runtime database, journal content, or server Docker configuration is included. A green preparation run does not imply registry publication.

## Publish the tested bundle

Use the existing local Linux workspace with `gh` authenticated and the verified `jdcloud` SSH alias. JD needs Python 3, Skopeo, and its existing registry login. Skopeo 1.13.3 was installed from JD's domestic Ubuntu mirror; no existing packages were upgraded or removed and SSH/relay/Docker stayed active.

From the repository root, use the run ID of the successful **Prepare release images** workflow:

```bash
python3 infra/relay_release.py RUN_ID
```

The command verifies trusted main workflow/source CI records, downloads the exact attempt's bundle through `gh`, and checks every file before using SSH. It transfers a checksum-bound archive with strict host-key checking, rejects unsafe extraction entries, verifies again on JD, and publishes through JD's existing credential file using Skopeo. Credentials are not copied to the workspace or GitHub.

Each target is `ccr.ccs.tencentyun.com/isaber/NAME:sha-COMMIT`. The registry digest must match the preserved source manifest and the remote configuration must match the tested image config. Only after all three images pass does the command retain:

```text
infra/releases/COMMIT/RUN_ID-ATTEMPT/release.json
```

That directory is ignored by Git. Keep this manifest for subsequent staging verification and promotion. It contains metadata and image references, not image bytes. JD's temporary archive/content and the local temporary download are cleaned afterward. The operator command never starts the blog or database.

## Failure handling

- Preparation failed: inspect the failing build/test/export step; no bundle is accepted for promotion.
- Artifact expired: rerun the preparation workflow's jobs and use the new attempt after it succeeds.
- SSH fails: inspect the existing alias and trusted host key; do not disable verification or expose SSH broadly.
- Integrity/provenance check fails: stop before publication and inspect the trusted source/artifact.
- One image copy fails: no completed release manifest is returned; retry the same verified bundle after diagnosing the cause. Already copied content can be reused.
- Direct mode is requested: set repository variable `TCR_PUBLISH_TRANSPORT=direct` explicitly. This retains the old route for controlled testing. Default/unset uses relay mode. If direct mode fails, unset the variable and rerun preparation before invoking the relay.

## Current evidence and remaining work

Twenty-one local safeguards passed. A real cached frontend/backend/PostgreSQL bundle exported and verified locally. The 284 MiB workspace-to-JD transfer took 185 seconds and passed extraction/full verification with publication deliberately omitted. JD also copied a previously verified public fixture through TCR with its digest unchanged. Those probes left only explicit diagnostic tags, not an application release.

The first real GitHub bundle download and all-three-image relay publication are pending the merged workflow. Fully unattended delivery, deployment connectivity, production retention/backups, TLS, and the final artistic frontend remain separate work.
