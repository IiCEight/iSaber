# Encrypted JD → Tencent backups

The owner approved Tencent as the backup vault and **03:30 Asia/Shanghai** as the daily time. `isaber-offsite.timer` runs on JD; no GitHub runner or workstation is needed for scheduled uploads. The existing local snapshot briefly stops only the blog writers, captures matching database/media/configuration, resumes the blog, and checks HTTPS. Other workloads and public ports are unchanged.

## Encryption and access

JD encrypts the private snapshot using `age` and the owner's public recipient. Only the workstation/offline recovery file holds the decryption identity. Neither cloud server nor GitHub has that identity. JD's separate signing key signs a manifest containing snapshot/release identity, plaintext/ciphertext byte counts, and SHA-256 receipts, using OpenSSH's `isaber-backup` namespace. The owner's recovery file includes a comment pinning the public signing key. Retain that comment when making copies.

Tencent stores ciphertext, the signed manifest, signature, and acknowledgement under `/var/lib/isaber-backup-vault/jd-staging/`. The dedicated `isaber-vault` SSH account has a root-owned authorized key with a forced command, no forwarding, and one exact bounded sudo receiver. It can add or verify a signed object, but cannot export, overwrite, delete, or open a shell. Operators retrieve ciphertext through their existing `ssh tencent`/sudo access. No new listener or cloud service is provisioned.

The receiver verifies the pinned signer and encryption recipient, enforces an immutable snapshot ID and full encrypted checksum, fsyncs receipt/data, then publishes the completed directory. A retry checks the committed object before transferring. JD retains the same ciphertext in its private outbox, so encryption randomness cannot cause conflicting retries. Pending failed work is retried on the next service start; the timestamp in the snapshot ID is its capture time, not its later upload time.

## Operator commands

Check schedule/result from the workstation:

```bash
ssh jdcloud 'systemctl list-timers isaber-offsite.timer --all --no-pager'
ssh jdcloud 'systemctl show isaber-offsite.service -p Result -p ExecMainStatus'
ssh jdcloud 'cat /opt/isaber-staging/offsite/status.json'
ssh jdcloud 'journalctl -u isaber-offsite.service -n 20 --no-pager'
```

`status.json` reports uploaded/failed state and a safe snapshot identifier; it contains no database or key contents. Check the identifier's capture timestamp for freshness. An uploaded acknowledgement establishes storage, not a new restore rehearsal. The historical restore proof is separate at `/opt/isaber-staging/offsite/restore-proof.json`, bound to the snapshot's `backup.json` checksum.

Take an additional backup or retry pending work:

```bash
ssh jdcloud 'systemctl start isaber-offsite.service'
```

It shares the existing operator lock with promotion/local recovery. A conflicting operation fails rather than running two coordinated captures. The timer uses its explicit timezone, `Persistent=true`, and a 45-minute service deadline. Stop/timeout signals raise through snapshot cleanup to resume paused blog services; systemd allows eight minutes for that cleanup before forcing termination. Hard power loss or forced kill still requires an operator health check. After an outage, a missed timer can run when JD returns. Disable future runs with `systemctl disable --now isaber-offsite.timer`; this does not cancel an already running service or delete backups.

JD's root-only `/opt/isaber-staging/offsite/` contains upload/signing keys, the public recipient/configuration, a pinned Tencent host key, outbox, and status. The decryption identity is deliberately absent. The age package was installed from JD's existing signed domestic Ubuntu mirror, with no unrelated package upgrades.

## Check a retrieved copy

Keep `/home/saber/.local/share/isaber/recovery/jd-staging.agekey` private (0600), together with its signer comment, and retain a protected copy outside the workstation. The owner confirmed that copy was saved. Never commit/upload this file or paste it into chat. A future JD sender rebuild must deliberately provision a new upload/signing identity and update vault/owner trust; the existing offline key alone cannot upload.

From this checkout on the workstation, choose a completed snapshot ID from `status.json`:

```bash
python3 infra/staging/offsite_recover.py \
  --id SNAPSHOT_ID \
  --identity /home/saber/.local/share/isaber/recovery/jd-staging.agekey \
  --age /home/saber/.local/libexec/isaber/age-1.1.1-ubuntu24.04.3/age \
  --keygen /home/saber/.local/libexec/isaber/age-1.1.1-ubuntu24.04.3/age-keygen
```

These workstation binaries were extracted from the signed Ubuntu age package without a system installation. On another workstation, install age through its trusted package manager and use the corresponding `--age` and `--keygen` paths. Existing operator SSH aliases must be available; `--vault` and `--target` can select alternatives.

The command retrieves only an encrypted export from Tencent, verifies its signature with the owner's pinned verifier, checks encrypted bytes, and decrypts locally. Plaintext streams over the operator's existing SSH connection to JD; the recovery identity stays local. The installed importer checks the signed plaintext hash and inventory before restoring. Temporary plaintext on JD is protected by root-only directories and removed after the check; the workstation keeps only temporary encrypted files, also removed afterward.

The restore uses a new **network-isolated PostgreSQL container**, constrained CPU/memory/processes, fresh database/media volumes, and the currently installed trusted PostgreSQL image. It never restores SQL into the live cluster and never executes backed-up Python/configuration. It compares table names/counts/content hashes and upload byte hashes, writes a safe proof, and removes the disposable resources. Retrieved archive headers are bounded and reject traversal, links, duplicate files, extensions, and special files before extraction.

## Verified acceptance

On November 9, 2026, snapshot `20261109T065908Z-e351832d` from application release `32cea61ba29defa610bf24937697669e407d283a` was encrypted on JD, uploaded to Tencent, retrieved, signature/hash checked, decrypted on the workstation, and restored into isolated JD resources. Database rows, database contents, and media bytes matched. The retry returned the existing immutable object, and an upload-key shell request was rejected. The live owner/article counts remained one each, HTTPS passed, and no disposable check containers or volumes remained. The first scheduled run is November 10 at 03:30 China time.

## Capacity and recovery limits

There is **no automatic deletion** in this milestone. Tencent has a 10 GiB encrypted-data quota and preserves at least 5 GiB free before accepting a new object. Ciphertext is capped at 2 GiB per object and its plaintext archive at 4 GiB. JD checks for 5 GiB free before capture and additional encryption working space. These admission checks are not filesystem quotas against unrelated host workloads or growth during a dump. Existing local snapshots/outbox objects remain until deliberate operator maintenance. Review capacity and backup freshness; an error preserves older copies and fails the service.

Scheduled failures are visible through systemd and `status.json`; external failure notifications and a reviewed retention policy are follow-up work. Pre-promotion recovery remains the existing local snapshot/restore gate; this daily offsite service does not add a Tencent availability dependency to deployment.

This rehearsal proves offsite database/media recovery, not a replacement-host rebuild or live data failover. Private secrets and certificate storage are encrypted in the snapshot, but rebuilding host access, restoring application roles/secrets/CA, and switching live volumes still require a reviewed maintenance procedure. No production RPO/RTO commitment is made yet.
