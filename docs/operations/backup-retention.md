# Verified backup retention

Retention is an operator-controlled process on each server. Upload and monitoring keys cannot invoke cleanup or change its policy. The dry-run planner is installed; activation requires owner approval of the periods below. Current backup contents remain unchanged by planning.

## Proposed policy

| Location | Copies retained |
| --- | --- |
| Tencent encrypted vault | All copies received in the last seven days; newest capture per China calendar day for the last 30 days, per ISO week for the last eight weeks, and per month for the last 12 months. Keep at least the newest three captures. |
| JD local snapshots | All captures from the last seven days and at least the newest ten snapshots. Also protect the latest available snapshot for each of the current/previous application releases. |
| JD encrypted outbox | All copies received in the last seven days and at least the newest ten copies. Protect the last successful offsite capture and pending work. |
| Both servers | Never automatically remove explicitly pinned recovery points. The snapshot successfully retrieved/decrypted/restored on November 9 is the initial pin, bound to its `backup.json` checksum. |

The calendar buckets use Asia/Shanghai (UTC+8); weekly buckets begin Monday. Missing days/weeks/months cannot be manufactured. Multiple rules can retain the same capture, so counts are not added together. Seven-day protection on Tencent uses its own receive time, protecting recently received copies even if the sender gives an old capture ID.

A copy outside these rules is first moved to a private **seven-day quarantine** on the same disk. Only a subsequent successful retention run after the full seven days may permanently delete it. Quarantine is an atomic rename, not another copy. Quarantined encrypted bytes still count against Tencent's existing 10 GiB quota and occupy real disk space. Retention does not create free space until purge.

A small permanent tombstone prevents the uploader from reusing a retired snapshot ID, including after ciphertext deletion. Operators can restore an original quarantined object under the same ID; the tombstone only blocks a new upload into a missing retired ID. Tombstones and the audit log contain identifiers/checksums, not article data or encryption identities.

## Safety gates

Every run plans under the existing JD backup/promotion lock or Tencent vault lock. Vault exports now take a shared lock so cleanup cannot remove an object during download. Cleanup never runs as part of an upload or from its forced SSH command.

Before any quarantine/purge:

- Verify the backup inventory and actual file/ciphertext hashes. On Tencent also verify signatures, recipient, and received receipts.
- Require an offsite capture less than 26 hours old and matching restored recovery pins. JD additionally requires the current backup health check to pass and a matching database/media restore proof.
- Preserve pending/last-success IDs and matching current/previous release points on JD. Refuse cleanup during a pending promotion.
- Reject unsafe names, symlinks, special files, future timestamps, changed pinned identities, invalid metadata, and incomplete quarantine records. Incomplete `.pending-*`/`.incoming-*` work is left to operators.
- Compute and execute a fresh plan while holding the same lock. Do not accept an uploaded list of paths or a stale plan file as deletion authority.

Failures preserve remaining copies and report a nonzero service result. A partially completed quarantine leaves its metadata for operator review; a partial purge affects only an already expired quarantine object. No live database, application upload volume, current secrets, image cache, or unrelated host directory enters the action list.

## Inspect a real plan

From the workstation:

```bash
ssh jdcloud 'python3 -I /opt/isaber-staging/config/retention.py --site jd'
ssh tencent 'sudo -n python3 -I /usr/local/lib/isaber-vault/retention.py --site tencent'
```

Without `--apply`, the commands only validate and record a plan. They do not move or delete backups. Plans list safe IDs, object kinds, checksums, and byte sizes, grouped into `keep`, `quarantine`, and `purge`.

The November 9 dry run kept all **ten local snapshots, one encrypted JD outbox copy, and one Tencent copy**. It proposed zero quarantine operations and zero deletions. Both policies were installed with `enabled: false` and both retention timers disabled. The restored pin is `20261109T065908Z-e351832d`, backup checksum `389ac94bc30bea93f7aa487376277e25d9e7b42985a4644524a5522b26599de8`.

Each server has a root-only `retention/` directory under its backup root (`/opt/isaber-staging` on JD, `/var/lib/isaber-backup-vault` on Tencent):

- `policy.json`: version, enabled flag, and pinned snapshot-ID → backup-checksum map.
- `last-plan.json`: latest computed inventory/actions.
- `status.json`: latest planned/applied/failed state, timestamp, and counts or safe exception type.
- `last-apply.json`: last actual apply result/freshness, kept separate so a dry run cannot overwrite the monitored job result.
- `events.jsonl`: fsynced quarantine/purge audit events.

Pins are an operator trust decision. Before changing a pin, retrieve/decrypt/restore the selected copy and verify its proof. Keep the older pin until the new one is accepted. Do not let the JD upload identity update Tencent's pins.

## Activation after policy approval

Through existing operator access, set `enabled: true` in each root-owned policy, run one `--apply` invocation against the freshly recomputed plan, then enable these timers:

```bash
ssh tencent 'sudo -n systemctl enable --now isaber-retention-vault.timer'
ssh jdcloud 'systemctl enable --now isaber-retention-jd.timer'
```

Tencent runs at **04:45 China time**, JD at **05:15**, after the 03:30 capture and its timeout/cleanup window. Each retention service has a 20-minute deadline and lowers CPU/I/O scheduling priority. The timers are persistent after outages, and stale/unhealthy backups still prevent cleanup. They may fail safely on lock contention rather than racing an operator or backup.

Inspect services/timers/status:

```bash
ssh tencent 'sudo -n systemctl list-timers isaber-retention-vault.timer --all --no-pager; sudo -n cat /var/lib/isaber-backup-vault/retention/status.json'
ssh jdcloud 'systemctl list-timers isaber-retention-jd.timer --all --no-pager; cat /opt/isaber-staging/retention/status.json'
```

Stopping future runs uses `systemctl disable --now` for the corresponding timer. Set the policy's enabled flag false to reject subsequent apply attempts too. This does not undo an already completed purge.

## Recover a quarantined copy

Quarantine entries live at `retention-trash/KIND/SNAPSHOT_ID/` under the corresponding root, with `quarantined.json` and an `object/` directory containing the original backup. Never rename individual recovered files into a live database or uploads volume.

Stop that retention timer, take its existing backup/vault lock, validate the quarantine receipt and object hashes, ensure the original active destination is absent, and move `object/` back to the original namespace. Remove the now-empty quarantine wrapper/receipt, fsync both parents, and record the operator recovery in the audit log. Add a policy pin if it must remain protected. Release the lock and perform the documented isolated restore check before considering a live recovery. This is an operator-only maintenance action; upload/monitor keys cannot do it.

A purged ciphertext/local snapshot cannot be recovered from that server's quarantine. Other retained copies and offline recovery material must be used. The offline decryption identity is never removed or rotated by retention.

## Capacity and monitoring boundaries

The rules preserve time coverage; they do not guarantee a fixed disk footprint. Large images or frequent snapshots may exceed JD space or Tencent's 10 GiB quota even with retention. Keep the five-GiB reserve and [existing backup alerts](backup-alerts.md); increase capacity or explicitly review policy rather than automatically weakening recovery protection.

Once a policy is enabled, the hourly backup monitor also checks its retention timer, service result, and `last-apply.json` freshness (26 hours). Tencent returns only safe enabled/healthy booleans with the already restricted ciphertext-verification reply; no new SSH operation or privilege is granted. Disabled policies are intentionally not treated as failed maintenance. An unhealthy retention job does not block its own recovery attempt: retention ignores maintenance-only alert codes while still requiring all backup/integrity/recovery gates to pass.

Failures remain visible in systemd and `retention/status.json`. During capacity incidents, JD's backup-health gate can refuse cleanup too; do not depend on retention as emergency disk recovery. Review capacity deliberately. Audit/tombstone log maintenance and full-host disaster rebuild remain follow-up work. Backups and deployment continue independently of these cleanup timers.
