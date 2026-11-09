# ADR 0016: Public-key encrypted backups in an append-only Tencent vault

Status: Implemented and cloud-tested for review.

## Decision

Use JD's existing coordinated snapshot and a systemd timer at 03:30 Asia/Shanghai. Encrypt on JD with the owner's public age recipient; keep its private identity only on the workstation/offline owner copy. Sign a strict checksum manifest with a separate JD Ed25519 key and pin its verifier both on Tencent and in the owner's recovery file.

Use the existing Tencent server as a root-controlled ciphertext vault over restricted SSH. The upload key may append/verify immutable objects only. Export uses the existing operator identity. Bound transfer size, capacity, idle time, and runtime; fail without deleting history. Reuse persisted ciphertext on retries. No new cloud product or public port is needed.

## Recovery verification

Retrieve ciphertext through operator SSH, authenticate it against the offline pinned signer, decrypt locally, and stream the signed-digest-bound plaintext into a disposable JD recovery check. Restore SQL only into a fresh network-isolated, resource-constrained PostgreSQL container with fresh volumes and a trusted installed image. Compare all table content hashes and media bytes, then remove disposable resources. Never execute copied control programs or overwrite live data during rehearsal.

The actual Tencent round trip passed on November 9 with an existing owner and article. The owner confirmed a protected copy of the recovery identity. See the [runbook](../../operations/offsite-backups.md) for evidence and commands.

## Consequences

Uploads are unattended without giving either server decryption access. The signing key makes an encrypted replacement created by a compromised vault detectable. A compromised JD host still has live plaintext and its signing key; this is not protection from source-host compromise. Existing immutable backups resist deletion through the uploader credential.

Recovery depends on retaining the offline identity and pinned signer. Deliberate key rotation/rebuild procedures, retention/pruning, external failure alerts, full-host disaster rebuild, and production recovery objectives remain future work. Daily offsite storage does not replace the existing local promotion/restore gate or make deployment depend on vault availability.
