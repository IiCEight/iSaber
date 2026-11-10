# Deliver tested images through JD Cloud

PR #14 established the first real release: frontend, backend, and PostgreSQL were published to TCR, pulled on JD by exact digest, and checked against their tested configurations. The operator recovery route remains `GitHub → workspace → JD → TCR`. [Automatic staging](automatic-staging.md) uses the restricted GitHub-to-JD cached transfer merged in PR #18. See [ADR 0012](../architecture/decisions/0012-image-bundle-relay.md).

## What is automatic

Push-to-main CI builds and tests the application images with the pinned official PostgreSQL baseline, then exports them as `ci-images-COMMIT-CI_ATTEMPT`. After CI succeeds, **Prepare release images** verifies and reuses that exact artifact without rebuilding or re-exporting images. It retains application blobs, source/CI metadata, and reviewed publishing tooling as `bundle-COMMIT-PREPARATION_ATTEMPT`, plus a small `metadata-COMMIT-PREPARATION_ATTEMPT` artifact. See [tested image reuse](automatic-staging.md#tested-image-reuse) for provenance and expiry rules.

Version 2 bundles reuse the existing TCR PostgreSQL digest from [`postgres-baseline.json`](../../infra/postgres-baseline.json). Export checks that the database configuration tested on GitHub matches this baseline. Database layers are omitted from the artifact; publication verifies the existing TCR configuration instead of uploading PostgreSQL. A missing or mismatched baseline fails the release. It never silently chooses a different database image.

Artifacts are retained for **one day** and use GitHub artifact storage allowance. No runtime database, journal content, or server credential configuration is included. A green preparation run means the bundle is ready; it does not imply publication or deployment.

## Publish the tested bundle

Use the existing Linux workspace with authenticated `gh` and the trusted `jdcloud` SSH alias. JD needs Python 3, Skopeo, and its existing private registry login. From the repository root:

```bash
python3 infra/relay_release.py RUN_ID
```

The default workspace route validates the trusted main workflow, successful source CI, current attempt, file hashes, image configurations/layers, and database provenance. It sends a checksum-bound archive through SSH with strict host-key checking. JD verifies it again before publishing with Skopeo through its existing registry credential file.

Frontend/backend receive `ccr.ccs.tencentyun.com/isaber/NAME:sha-COMMIT` tags. PostgreSQL retains its pinned existing digest. Every remote image configuration must match the tested configuration before a completed manifest is saved:

```text
infra/releases/COMMIT/RUN_ID-ATTEMPT/release.json
```

Keep this ignored local manifest for deployment and rollback. Temporary downloads and staging content are removed afterward. The operator command never starts the blog or database.

## Optional direct artifact download on JD

```bash
python3 infra/relay_release.py RUN_ID --transport server
```

The workspace downloads only the small metadata artifact and validates its source CI. It obtains a fresh, short-lived GitHub artifact URL and sends that URL through encrypted SSH stdin. The GitHub token remains local; the URL is not printed, put in process arguments, or written to a configuration file. JD checks the complete archive against GitHub's SHA-256/size receipt, rejects unsafe ZIP entries, and compares its metadata before verifying the bundle and publishing. No persistent GitHub credential or runner is installed on JD. GitHub documents the [artifact download redirect](https://docs.github.com/en/rest/actions/artifacts#download-an-artifact).

This is opt-in because the live JD probe received only **2 MiB in 72 seconds**. Removing the workspace currently makes delivery slower. A direct download is bounded to ten minutes, with a socket timeout; network errors omit signed URLs. If it fails, retry through the default workspace route. Older runs lacking the separate metadata artifact use the workspace route, including version 1 bundles containing PostgreSQL layers.

## Upgrading PostgreSQL

The baseline is a deliberate dependency pin, not a permanent freeze. For each planned PostgreSQL security/maintenance upgrade:

1. Select the official upstream digest and mirror its linux/amd64 image into the existing TCR PostgreSQL repository through a working network path, preserving the destination manifest digest.
2. Pull and verify that exact TCR digest against the official image configuration and test the database executable. Keep the old digest for rollback.
3. Update the upstream digest in `compose.rehearsal.yml` and `publish_images.py`, and the upstream, destination, and configuration digests in `postgres-baseline.json`, together in a reviewed PR.
4. Pass container/migration acceptance on the new baseline before promoting an application release. Export refuses a tested configuration that differs from the pin.

If the existing baseline is removed from TCR, restore the verified image before retrying; the relay does not substitute a mutable `postgres:16` tag.

## Failure handling

- Failed or expired preparation: inspect the failure or rerun preparation and use the successful new attempt.
- SSH failure: inspect the existing alias and trusted host key; keep host verification enabled.
- Integrity/provenance failure: stop before publication and inspect the trusted source/artifact.
- Partial image-copy failure: no completed manifest is returned; retry after diagnosis. Existing registry blobs can be reused.
- Optional direct GitHub-to-TCR publication: `TCR_PUBLISH_TRANSPORT=direct` publishes the same reused verified bundle with an ephemeral registry login. This is separate from `--transport server`, which changes how JD receives the artifact. Both paths retain the verified database baseline. Failed direct publication still requires switching back to relay mode and rerunning successful preparation.

## Timing and remaining work

The first release's GitHub CI took 1m 51s and preparation took another 1m 51s. Its 283 MiB workspace-to-JD transfer took 170 seconds, followed by registry upload and first pull verification. The reduced real bundle is **124.3 MiB**, with no database image layers, a reduction of about 56%. A real reduced-bundle transfer took **75.5 seconds** and passed full verification on JD; the existing TCR database configuration also matched. This probe did not publish fixture images or start application services.

The current milestone removes duplicate image builds across CI/preparation. Unattended private staging/TLS, encrypted backups, and isolated database/media restore are implemented; replacement-server/full-host recovery, production delivery, and remaining artistic page design still need acceptance. Direct download has bounded tests and a live throughput probe; full production artifact delivery through that optional route is not yet accepted.
