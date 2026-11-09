# Staging snapshots and disposable restore checks

The private JD staging project has coordinated snapshots and verified local recovery. [Encrypted offsite backups](offsite-backups.md) also run daily at 03:30 China time, from JD to a restricted vault on Tencent.

## Take a snapshot

Run from the workstation:

```bash
ssh jdcloud 'python3 /opt/isaber-staging/config/backup.py snapshot'
```

The command takes an exclusive operator lock and briefly stops only the staging edge, frontend, and API. This freezes application database/media writes while PostgreSQL remains available for its logical dump. Avoid simultaneous owner/migration commands during this window. It records a PostgreSQL custom-format dump, upload archive, certificate archive, server secret file, image/release metadata, and deployment configuration in a new private directory under `/opt/isaber-staging/backups/`.

Table row counts and deterministic hashes of sorted row JSON record the database contents. Every archived upload has its own byte hash; all snapshot files have size/hash receipts. The command flushes file/directory state before marking the snapshot complete. Finally it resumes only the blog services that were previously running and verifies HTTPS. On a failure, the partial snapshot is removed and service resumption is still attempted.

Snapshots contain **database credentials, password hashes, session data, media, and the private staging CA keys**. They remain on JD in the root-only staging tree. They are not copied to the workspace, GitHub, or the public repository. Their file mode is 0600 and snapshot directories use 0700. Keep them in protected storage; do not attach the directory to an issue or artifact upload.

## Verify recovery

Use the completed directory printed by the snapshot command:

```bash
ssh jdcloud 'python3 /opt/isaber-staging/config/backup.py restore-check --snapshot /opt/isaber-staging/backups/SNAPSHOT_ID'
```

The command verifies the snapshot inventory before creating anything. It restores into a new randomly named `isaber_restore_*` database and `isaber-restore-*` upload volume. Neither restore target is the live database or uploads volume. It compares table names/counts/content hashes and exact media bytes, writes a private `restore-proof.json` bound to `backup.json`, then removes its disposable database and volume. Restore helpers have no network access and mount live upload data read-only during capture.

Archive validation rejects traversal, symlinks, hard links, duplicate paths, and special files. The test fails if any receipt, database content hash, or media hash differs. Concurrent snapshot/promotion/recovery operations are rejected by the shared operator lock. An interrupted promotion must be recovered before another snapshot is taken.

## Completed cloud acceptance

A first clean snapshot restored successfully. A second rehearsal created a uniquely named synthetic table row and file, captured and restored them, and checked the actual nonempty contents. It removed those live probes, then took and restored a clean snapshot at:

```text
/opt/isaber-staging/backups/20261108T151526Z-3e55ad91
```

Application HTTPS resumed after each snapshot. The source database/media were never restored over. Later promotion rehearsals take additional recovery points automatically; this path is evidence from the initial clean rehearsal, not a moving latest-backup alias.

## Recovery boundary

`restore-check` proves a local logical database/media restore. It does not switch the live application to the disposable targets. Live data restoration requires a deliberate maintenance plan: stop writers, choose and verify a recovery point, restore into replacement database/media volumes, provision the operator/runtime roles, restore matching secrets and CA storage, verify schema and privacy, then switch the dedicated project. There is no automatic data rewind during application rollback.

Local snapshots remain on JD. Encrypted offsite copies now protect against losing that disk, and a retrieved copy has passed an isolated database/media recovery check. Retention/pruning, full-host disaster-rebuild acceptance, failure notifications, and production recovery objectives remain follow-up work. Staging now contains the owner's account and an article; preserve them during operational rehearsals.
