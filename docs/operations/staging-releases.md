# Promote and roll back private staging releases

Initial deployment uses `deploy`; subsequent updates use the guarded operator `promote` command. The existing private HTTPS endpoint, secrets, database, uploads, and CA persist across application releases. These operations are not yet triggered automatically by GitHub.

## Promote a verified release

First run the existing [image relay](image-relay.md) for a successful trusted-main preparation run. It validates workflow/source CI, publishes the tested image digests, and retains a verified `release.json` locally. Copy only that manifest to the private staging directory:

```bash
scp infra/releases/COMMIT/RUN_ID-ATTEMPT/release.json jdcloud:/opt/isaber-staging/candidate-release.json
ssh jdcloud 'python3 /opt/isaber-staging/config/staging.py promote --release /opt/isaber-staging/candidate-release.json'
```

The operator command:

1. Takes the same exclusive lock used for snapshots and refuses an unfinished earlier promotion.
2. Requires the same reviewed schema version 5, API contract hash, and exact PostgreSQL image as the current release. It checks the live migration version too.
3. Pulls the candidate application digests and verifies their actual configuration hashes against the release manifest.
4. Takes and restores a coordinated recovery snapshot before changing application references.
5. Writes a private, atomic pending-promotion journal containing the old image environment and release/history metadata.
6. Checks the candidate migration status. Every reviewed migration must already be applied; promotion never runs `migrate up` or applies a schema change.
7. Briefly stops and replaces only the blog application/proxy containers, checks readiness and CA-verified HTTPS routes, and then records current/previous release metadata.

This single-instance deployment has a maintenance interruption for the snapshot and application switch. It does not promise zero downtime on the shared 2 GiB host. Owner/password/media/database state is preserved; a release update does not rerun owner setup or rotate secrets.

A changed schema, API contract, or PostgreSQL baseline needs a separate compatibility/migration review before this narrow promotion policy is extended. A manifest is accepted from the existing trusted operator relay; do not substitute an unverified hand-written file.

## Roll back application images

```bash
ssh jdcloud 'python3 /opt/isaber-staging/config/staging.py rollback'
```

This promotes `previous-release.json` through the same compatibility, image, recovery-snapshot, and health gates. After success, the two releases become current/previous in the opposite order, allowing a deliberate return forward.

If a candidate fails validation after its environment is staged, startup, or HTTPS acceptance, the command automatically recovers the journaled prior image references and verifies HTTPS again. The promotion still reports failure. If recovery fails, the journal remains for diagnosis and further changes are blocked.

Application rollback **retains current database and media state**. It does not restore a snapshot over live data, discard new articles, or downgrade schema. Recovery snapshots are for separately reviewed data recovery.

## Recover an interrupted promotion

```bash
ssh jdcloud 'python3 /opt/isaber-staging/config/staging.py recover'
```

If a pending journal exists, this restores the prior image environment and release/history metadata, starts the application services, and checks HTTPS. It removes the journal only after successful recovery. If no journal exists it makes no changes. Metadata writes flush file/directory state so recovery can identify interrupted updates. Keep the pending journal and private snapshot intact while diagnosing a failure.

## Acceptance and remaining work

The cloud rehearsal uses the two already verified releases from PR #14 and PR #15. It checks a real application promotion, explicit rollback, and an intentionally failed candidate health gate followed by automatic recovery. The intended final release is `8cca5a4310627f9585bef4c845db278007729754`, with the same database, media, secrets, and HTTPS CA. Rehearsal results are recorded privately in `/opt/isaber-staging/promotion-rehearsal.json` after completion.

Unit guards cover incompatible contract/database/schema rejection, pending-journal rejection, restore-proof failure before application changes, normal release metadata/history, migration status without migration execution, and recovery after a failed health gate.

Next is a restricted deployment identity and a measured, authenticated GitHub-to-server delivery path. No new server SSH key is copied to GitHub and no unattended job is installed by this milestone.
