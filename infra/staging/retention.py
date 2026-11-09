"""Operator-only retention: verified plans, protected recovery pins, quarantine."""
import argparse
from contextlib import contextmanager
from datetime import datetime, timedelta, timezone
import fcntl
import hashlib
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import sys
import tempfile

sys.path.insert(0, str(Path(__file__).resolve().parent))
from vault_protocol import validate_id, validate_manifest

CHINA = timezone(timedelta(hours=8))
QUARANTINE_DAYS = 7
MAX_ITEMS = 4096


def digest(path):
    value = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""): value.update(block)
    return value.hexdigest()


def sync(path):
    descriptor = os.open(path, os.O_RDONLY | os.O_DIRECTORY)
    try: os.fsync(descriptor)
    finally: os.close(descriptor)


def write(path, value):
    temporary = path.with_suffix(path.suffix + ".pending")
    with temporary.open("w") as output:
        temporary.chmod(0o600)
        json.dump(value, output, sort_keys=True, indent=2); output.write("\n")
        output.flush(); os.fsync(output.fileno())
    temporary.replace(path); sync(path.parent)


def read(path):
    if path.is_symlink() or path.stat().st_size > 65536: raise ValueError("Invalid retention metadata")
    value = json.loads(path.read_text())
    if not isinstance(value, dict): raise ValueError("Expected a metadata object")
    return value


def timestamp(value):
    result = datetime.fromisoformat(value)
    if result.tzinfo is None: raise ValueError("Require a timezone")
    return result.astimezone(timezone.utc)


def safe_tree(path):
    if path.is_symlink() or not path.is_dir() or path.resolve() != path.absolute(): raise ValueError("Unsafe backup directory")
    total = 0
    for item in path.rglob("*"):
        if item.is_symlink() or not (item.is_file() or item.is_dir()): raise ValueError("Unsafe backup member")
        if item.is_file(): total += item.stat().st_size
    return total


def policy(root):
    value = read(root / "retention/policy.json")
    if set(value) != {"version", "enabled", "pins"} or type(value["version"]) is not int or value["version"] != 1 or type(value["enabled"]) is not bool or not isinstance(value["pins"], dict) or not value["pins"]:
        raise ValueError("Require an explicit policy and at least one restored recovery pin")
    import re
    for identifier, checksum in value["pins"].items():
        validate_id(identifier)
        if not isinstance(checksum, str) or not re.fullmatch(r"[0-9a-f]{64}", checksum): raise ValueError("Invalid pinned backup checksum")
    return value


def select(records, site, now, pins, protected=()):
    """Pure policy: no caller-supplied path can enter a cleanup action."""
    records = sorted(records, key=lambda item: (item["captured"], item["id"]), reverse=True)
    keep = set(pins) | set(protected)
    keep.update(item["id"] for item in records[:3 if site == "tencent" else 10])
    for item in records:
        if item["captured"] > now + timedelta(minutes=5) or item["received"] > now + timedelta(minutes=5):
            raise ValueError("Future backup timestamp; cleanup refused")
        if now - item["received"] < timedelta(days=7): keep.add(item["id"])
    if site == "tencent":
        today = now.astimezone(CHINA).date()
        week_start = today - timedelta(days=today.weekday())
        month_number = today.year * 12 + today.month - 1
        seen = {"day": set(), "week": set(), "month": set()}
        for item in records:
            date = item["captured"].astimezone(CHINA).date()
            week = date - timedelta(days=date.weekday())
            month = date.year * 12 + date.month - 1
            for label, key, eligible in (("day", date, date >= today - timedelta(days=29)), ("week", week, week >= week_start - timedelta(weeks=7)), ("month", month, month >= month_number - 11)):
                if eligible and key not in seen[label]:
                    seen[label].add(key); keep.add(item["id"])
    return keep


def directories(parent):
    if not parent.exists(): return []
    if parent.is_symlink(): raise ValueError("Symlinked backup tree")
    result = []
    for path in parent.iterdir():
        # Incomplete captures/transfers are never automatically cleaned here.
        if path.name.startswith("."): continue
        validate_id(path.name)
        if path.is_symlink() or not path.is_dir(): raise ValueError("Unsafe backup namespace")
        result.append(path)
        if len(result) > MAX_ITEMS: raise ValueError("Backup inventory exceeds the review limit")
    return result


def vault_records(root):
    import vault
    records = []
    with tempfile.TemporaryDirectory(prefix=".retention-check-", dir=root) as temporary:
        for directory in directories(root / "jd-staging"):
            size = safe_tree(directory)
            if {path.name for path in directory.iterdir()} != {"manifest.json", "signature", "receipt.json", "snapshot.age"}: raise ValueError("Unexpected vault members")
            manifest = validate_manifest(read(directory / "manifest.json"))
            receipt = read(directory / "receipt.json")
            vault.check_signature(manifest, (directory / "signature").read_text(), Path(temporary), root=root)
            if manifest["id"] != directory.name or receipt.get("manifest") != manifest or receipt.get("status") != "stored" or (directory / "snapshot.age").stat().st_size != manifest["cipher_bytes"] or digest(directory / "snapshot.age") != manifest["cipher_sha256"]:
                raise ValueError("Vault integrity mismatch; cleanup refused")
            records.append({"id": directory.name, "captured": validate_id(directory.name), "received": timestamp(receipt["received_at"]), "backup_sha256": manifest["backup_sha256"], "identity": digest(directory / "manifest.json"), "bytes": size})
    return records


def jd_records(root):
    from backup import verify_snapshot
    local, outbox = [], []
    for directory in directories(root / "backups"):
        size = safe_tree(directory)
        metadata = verify_snapshot(root, directory)
        local.append({"id": directory.name, "captured": validate_id(directory.name), "received": validate_id(directory.name), "backup_sha256": digest(directory / "backup.json"), "identity": digest(directory / "backup.json"), "source_commit": metadata["release"]["source_commit"], "bytes": size})
    for directory in directories(root / "offsite/outbox"):
        size = safe_tree(directory)
        if {path.name for path in directory.iterdir()} != {"manifest.json", "signature", "receipt.json", "snapshot.age"}:
            raise ValueError("Unexpected outbox members; cleanup refused")
        manifest = validate_manifest(read(directory / "manifest.json"))
        receipt = read(directory / "receipt.json")
        if manifest["id"] != directory.name or receipt.get("manifest") != manifest or receipt.get("status") not in ("stored", "exists") or (directory / "snapshot.age").stat().st_size != manifest["cipher_bytes"] or digest(directory / "snapshot.age") != manifest["cipher_sha256"]:
            raise ValueError("Outbox integrity mismatch; cleanup refused")
        outbox.append({"id": directory.name, "captured": validate_id(directory.name), "received": timestamp(receipt["received_at"]), "backup_sha256": manifest["backup_sha256"], "identity": digest(directory / "manifest.json"), "bytes": size})
    return local, outbox


def source_directory(root, kind, identifier):
    validate_id(identifier)
    parent = {"local": root / "backups", "outbox": root / "offsite/outbox", "vault": root / "jd-staging"}[kind]
    return parent / identifier


def trash_directory(root, kind, identifier):
    validate_id(identifier)
    if kind not in ("local", "outbox", "vault"): raise ValueError("Invalid backup kind")
    return root / "retention-trash" / kind / identifier


def plan(root, site, now=None):
    now = now or datetime.now(timezone.utc)
    configuration = policy(root)
    pins = configuration["pins"]
    if site == "tencent":
        groups = {"vault": vault_records(root)}
        verified = groups["vault"]
        protected = set()
    else:
        from backup_health import collect, evaluate
        if (root / "pending-promotion.json").exists():
            raise ValueError("Recover the pending promotion before retention")
        problems = set(evaluate(collect(root), now)) - {"local_retention_failed", "vault_retention_failed"}
        if problems:
            raise ValueError("Backup health or promotion safety gate failed; cleanup refused")
        success = read(root / "offsite/last-success.json")
        proof = read(root / "offsite/restore-proof.json")
        if proof.get("backup_sha256") not in pins.values() or any(proof.get(key) is not True for key in ("database_rows_match", "database_contents_match", "media_bytes_match", "isolated_container")):
            raise ValueError("Require a matching successful offsite restore proof")
        local, outbox = jd_records(root)
        groups = {"local": local, "outbox": outbox}
        verified = outbox
        protected = {success["snapshot_id"]}
        if (root / "offsite/pending.json").exists(): protected.add(read(root / "offsite/pending.json")["id"])
        # Keep a local recovery point for current and immediately previous images.
        for filename in ("current-release.json", "previous-release.json"):
            if (root / filename).exists():
                commit = read(root / filename)["source_commit"]
                matching = [item for item in local if item["source_commit"] == commit]
                if matching: protected.add(max(matching, key=lambda item: item["captured"])["id"])
    if not verified or now - max(item["captured"] for item in verified) > timedelta(hours=26):
        raise ValueError("Require a fresh verified offsite backup before cleanup")
    by_id = {item["id"]: item for item in verified}
    for identifier, checksum in pins.items():
        if identifier not in by_id or by_id[identifier]["backup_sha256"] != checksum:
            raise ValueError("Pinned restored copy missing or changed; cleanup refused")
    result = {"version": 1, "site": site, "checked_at": now.isoformat(), "enabled": configuration["enabled"], "keep": [], "quarantine": [], "purge": []}
    for kind, records in groups.items():
        keep = select(records, site, now, pins, protected)
        # All three kinds retain the restored recovery pin, if present locally.
        for item in records:
            action = {"kind": kind, "id": item["id"], "identity": item["identity"], "bytes": item["bytes"]}
            result["keep" if item["id"] in keep else "quarantine"].append(action)
        for directory in directories(root / "retention-trash" / kind):
            safe_tree(directory)
            if {path.name for path in directory.iterdir()} != {"quarantined.json", "object"}: raise ValueError("Incomplete quarantine; operator review required")
            record = read(directory / "quarantined.json")
            if set(record) != {"version", "kind", "id", "identity", "quarantined_at"} or type(record["version"]) is not int or record["version"] != 1 or record["kind"] != kind or record["id"] != directory.name:
                raise ValueError("Invalid quarantine receipt")
            quarantined = timestamp(record["quarantined_at"])
            if quarantined > now or directory.name in keep: raise ValueError("Protected/future quarantine; recover it before cleanup")
            identity = "backup.json" if kind == "local" else "manifest.json"
            if digest(directory / "object" / identity) != record["identity"]: raise ValueError("Quarantined identity mismatch")
            if now - quarantined >= timedelta(days=QUARANTINE_DAYS):
                result["purge"].append({"kind": kind, "id": directory.name, "identity": record["identity"], "bytes": safe_tree(directory)})
    return result


def maintenance_status(root, site, now=None):
    """Safe boolean summary; no policy pins or backup contents leave the server."""
    now = now or datetime.now(timezone.utc)
    if not (root / "retention/policy.json").exists(): return {"enabled": False, "healthy": True}
    try:
        configuration = policy(root)
        if not configuration["enabled"]: return {"enabled": False, "healthy": True}
        name = "isaber-retention-jd" if site == "jd" else "isaber-retention-vault"
        units = []
        for suffix in (".timer", ".service"):
            result = subprocess.run(["systemctl", "show", name + suffix, "-p", "LoadState", "-p", "ActiveState", "-p", "UnitFileState", "-p", "Result"], check=True, capture_output=True, timeout=5)
            units.append(dict(line.split("=", 1) for line in result.stdout.decode().splitlines() if "=" in line))
        timer, service = units
        applied = read(root / "retention/last-apply.json")
        age = (now - timestamp(applied["checked_at"])).total_seconds()
        healthy = timer.get("LoadState") == service.get("LoadState") == "loaded" and timer.get("ActiveState") == "active" and timer.get("UnitFileState") == "enabled" and service.get("Result") == "success" and applied.get("state") == "applied" and -300 <= age <= 26 * 3600
        return {"enabled": True, "healthy": healthy}
    except Exception: return {"enabled": True, "healthy": False}


def audit(root, event):
    path = root / "retention/events.jsonl"
    with path.open("a") as output:
        path.chmod(0o600); output.write(json.dumps(event, sort_keys=True) + "\n")
        output.flush(); os.fsync(output.fileno())


def apply(root, result):
    if not result["enabled"]: raise ValueError("Retention is not approved/enabled")
    if not shutil.rmtree.avoids_symlink_attacks: raise ValueError("Require symlink-safe recursive deletion")
    for action in result["quarantine"]:
        source = source_directory(root, action["kind"], action["id"])
        destination = trash_directory(root, action["kind"], action["id"])
        safe_tree(source)
        destination.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
        destination.mkdir(mode=0o700)
        write(destination / "quarantined.json", {"version": 1, "kind": action["kind"], "id": action["id"], "identity": action["identity"], "quarantined_at": result["checked_at"]})
        if action["kind"] == "vault":
            tombstones = root / "retention-tombstones/vault"
            tombstones.mkdir(mode=0o700, parents=True, exist_ok=True)
            write(tombstones / (action["id"] + ".json"), {"version": 1, "id": action["id"], "identity": action["identity"], "retired_at": result["checked_at"]})
        source.rename(destination / "object")
        sync(source.parent); sync(destination); sync(destination.parent)
        audit(root, {"operation": "quarantined", "at": result["checked_at"], **action})
    for action in result["purge"]:
        destination = trash_directory(root, action["kind"], action["id"])
        safe_tree(destination)
        audit(root, {"operation": "purge_started", "at": result["checked_at"], **action})
        shutil.rmtree(destination); sync(destination.parent)
        audit(root, {"operation": "purged", "at": result["checked_at"], **action})


@contextmanager
def locked(root, site):
    if site == "jd":
        from backup import lock
        with lock(root): yield
    else:
        with (root / ".lock").open("a") as handle:
            fcntl.flock(handle, fcntl.LOCK_EX | fcntl.LOCK_NB)
            yield


def interrupted(*_):
    raise TimeoutError("Retention interrupted; remaining copies preserved")


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--site", choices=("jd", "tencent"), required=True)
    parser.add_argument("--root", type=Path)
    parser.add_argument("--apply", action="store_true")
    args = parser.parse_args()
    root = args.root or Path("/opt/isaber-staging" if args.site == "jd" else "/var/lib/isaber-backup-vault")
    os.umask(0o077)
    for signum in (signal.SIGTERM, signal.SIGINT, signal.SIGHUP): signal.signal(signum, interrupted)
    try:
        if os.geteuid() != 0: raise ValueError("Use the existing privileged operator connection")
        with locked(root, args.site):
            result = plan(root, args.site)
            write(root / "retention/last-plan.json", result)
            if args.apply: apply(root, result)
            status = {"state": "applied" if args.apply else "planned", "checked_at": result["checked_at"], "kept": len(result["keep"]), "quarantined": len(result["quarantine"]), "purged": len(result["purge"])}
            write(root / "retention/status.json", status)
            if args.apply: write(root / "retention/last-apply.json", status)
            print(json.dumps(result), flush=True)
    except Exception as error:
        if (root / "retention").is_dir():
            status = {"state": "failed", "error_type": type(error).__name__, "checked_at": datetime.now(timezone.utc).isoformat()}
            write(root / "retention/status.json", status)
            if args.apply: write(root / "retention/last-apply.json", status)
        print("Retention refused/failed (" + type(error).__name__ + "); private output omitted", file=sys.stderr)
        raise SystemExit(1)
