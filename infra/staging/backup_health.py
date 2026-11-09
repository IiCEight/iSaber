"""Fixed read-only cloud check and strict, shared backup-health policy."""
from datetime import datetime, timezone
import json
from pathlib import Path
import shutil
import signal
import subprocess
import sys
import time

sys.path.insert(0, str(Path(__file__).resolve().parent))
from vault_protocol import validate_id, validate_manifest, validate_signature

ROOT = Path("/opt/isaber-staging")
MAX_AGE_HOURS = 26
MIN_FREE_BYTES = 5 * 1024**3
FIELDS = {"version", "checked_at", "timer_enabled", "timer_active", "service_active", "service_result", "last_attempt", "last_snapshot_id", "vault_verified", "disk_free_bytes", "pending", "errors"}
MESSAGES = {
    "timer_query_failed": "Cannot read the backup timer/service state.",
    "status_invalid": "Backup status or last-success metadata is missing or invalid.",
    "receipt_invalid": "The last successful snapshot has no matching signed receipt.",
    "storage_query_failed": "Cannot determine JD disk space.",
    "timer_disabled": "The daily backup timer is disabled.",
    "timer_inactive": "The daily backup timer is not active.",
    "backup_service_failed": "The last backup service run failed.",
    "last_attempt_failed": "The most recent backup attempt failed.",
    "snapshot_missing": "No successful offsite backup is recorded.",
    "backup_overdue": "The last captured offsite backup is more than 26 hours old.",
    "clock_skew": "The backup/server timestamp is unexpectedly in the future or the server clock differs by over five minutes.",
    "vault_unverified": "Tencent could not confirm the signed ciphertext and checksum.",
    "low_disk_space": "JD has less than 5 GiB free for backups.",
    "pending_backup": "A pending backup remains while the service is not running.",
    "local_retention_failed": "JD retention is enabled but its timer, last run, or freshness check failed.",
    "vault_retention_failed": "Tencent retention is enabled but its timer, last run, or freshness check failed.",
}


def timestamp(value):
    parsed = datetime.fromisoformat(value)
    if parsed.tzinfo is None: raise ValueError("Require a timezone")
    return parsed.astimezone(timezone.utc)


def evaluate(report, now=None):
    """The external monitor independently evaluates capture time against its clock."""
    now = now or datetime.now(timezone.utc)
    if not isinstance(report, dict) or set(report) != FIELDS or type(report["version"]) is not int or report["version"] != 1:
        raise ValueError("Invalid bounded health report")
    for field in ("timer_enabled", "timer_active", "vault_verified", "pending"):
        if type(report[field]) is not bool: raise ValueError("Invalid health boolean")
    if type(report["disk_free_bytes"]) is not int or not 0 <= report["disk_free_bytes"] <= 1024**6:
        raise ValueError("Invalid disk-space measurement")
    if report["service_active"] not in ("inactive", "active", "activating", "deactivating", "failed", "unknown") or report["service_result"] not in ("success", "exit-code", "signal", "timeout", "resources", "watchdog", "unknown") or report["last_attempt"] not in ("uploaded", "failed", "missing"):
        raise ValueError("Invalid service state")
    if not isinstance(report["errors"], list) or len(report["errors"]) > len(MESSAGES) or any(type(code) is not str or code not in MESSAGES for code in report["errors"]):
        raise ValueError("Invalid health error codes")
    problems = set(report["errors"])
    if abs((now - timestamp(report["checked_at"])).total_seconds()) > 300: problems.add("clock_skew")
    if not report["timer_enabled"]: problems.add("timer_disabled")
    if not report["timer_active"]: problems.add("timer_inactive")
    if report["service_result"] != "success" or report["service_active"] == "failed": problems.add("backup_service_failed")
    if report["last_attempt"] == "failed": problems.add("last_attempt_failed")
    if report["last_snapshot_id"] is None:
        problems.add("snapshot_missing")
    else:
        captured = validate_id(report["last_snapshot_id"])
        age = (now - captured).total_seconds()
        if age < -300: problems.add("clock_skew")
        if age > MAX_AGE_HOURS * 3600: problems.add("backup_overdue")
    if not report["vault_verified"]: problems.add("vault_unverified")
    if report["disk_free_bytes"] < MIN_FREE_BYTES: problems.add("low_disk_space")
    if report["pending"] and report["service_active"] not in ("active", "activating", "deactivating"): problems.add("pending_backup")
    return sorted(problems)


def read_json(path):
    if path.is_symlink() or path.stat().st_size > 65536: raise ValueError("Invalid protected metadata")
    value = json.loads(path.read_text())
    if not isinstance(value, dict): raise ValueError("Require a metadata object")
    return value


def unit_state(unit):
    result = subprocess.run(["systemctl", "show", unit, "-p", "LoadState", "-p", "ActiveState", "-p", "UnitFileState", "-p", "Result"], check=True, capture_output=True, timeout=10)
    state = dict(line.split("=", 1) for line in result.stdout.decode().splitlines() if "=" in line)
    if state.get("LoadState") != "loaded": raise ValueError("Backup unit is not loaded")
    return state


def verify_vault(root, success, maintenance=None):
    from offsite import ssh_arguments
    directory = root / "offsite/outbox" / success["snapshot_id"]
    manifest = validate_manifest(read_json(directory / "manifest.json"))
    if manifest["id"] != success["snapshot_id"] or manifest["cipher_sha256"] != success["cipher_sha256"]:
        raise ValueError("Last-success identity differs from signed receipt")
    signature = validate_signature((directory / "signature").read_text())
    header = (json.dumps({"operation": "verify", "manifest": manifest, "signature": signature}) + "\n").encode()
    # A concurrent upload can briefly own the vault's exclusive lock. Retry once
    # without uploading, rewriting status, taking snapshots, or changing units.
    for attempt in range(2):
        try:
            result = subprocess.run(ssh_arguments(root), input=header, capture_output=True, timeout=40)
            if result.returncode == 0 and len(result.stdout) <= 16384:
                receipt = json.loads(result.stdout)
                if receipt.get("status") == "exists" and receipt.get("manifest") == manifest:
                    state = receipt.get("retention")
                    if maintenance is not None and state is not None:
                        maintenance["healthy"] = isinstance(state, dict) and set(state) == {"enabled", "healthy"} and type(state.get("enabled")) is bool and type(state.get("healthy")) is bool and (not state["enabled"] or state["healthy"])
                    return True
        except (subprocess.TimeoutExpired, ValueError):
            pass
        if attempt == 0: time.sleep(3)
    return False


def collect(root=ROOT):
    report = {"version": 1, "checked_at": datetime.now(timezone.utc).isoformat(), "timer_enabled": False, "timer_active": False, "service_active": "unknown", "service_result": "unknown", "last_attempt": "missing", "last_snapshot_id": None, "vault_verified": False, "disk_free_bytes": 0, "pending": (root / "offsite/pending.json").exists(), "errors": []}
    try:
        timer, service = unit_state("isaber-offsite.timer"), unit_state("isaber-offsite.service")
        report.update(timer_enabled=timer.get("UnitFileState") == "enabled", timer_active=timer.get("ActiveState") == "active", service_active=service.get("ActiveState", "unknown"), service_result=service.get("Result", "unknown"))
    except Exception: report["errors"].append("timer_query_failed")
    try: report["disk_free_bytes"] = shutil.disk_usage(root).free
    except OSError: report["errors"].append("storage_query_failed")
    success = None
    try:
        latest = read_json(root / "offsite/status.json")
        if latest.get("state") not in ("uploaded", "failed"): raise ValueError("Invalid attempt state")
        report["last_attempt"] = latest["state"]
        successful = root / "offsite/last-success.json"
        # Upgrade compatibility: PR #19 has status.json but no separate success
        # record. A later failure cannot be mistaken for a successful capture.
        success = read_json(successful) if successful.exists() else latest
        if success.get("state") != "uploaded": raise ValueError("No prior success")
        validate_id(success["snapshot_id"])
        report["last_snapshot_id"] = success["snapshot_id"]
    except Exception:
        success = None; report["errors"].append("status_invalid")
    if success:
        maintenance = {}
        try:
            report["vault_verified"] = verify_vault(root, success, maintenance)
            if not maintenance.get("healthy", True): report["errors"].append("vault_retention_failed")
        except Exception: report["errors"].append("receipt_invalid")
    if (root / "retention/policy.json").exists():
        from retention import maintenance_status
        state = maintenance_status(root, "jd")
        if state["enabled"] and not state["healthy"]: report["errors"].append("local_retention_failed")
    return report


def interrupted(*_):
    raise TimeoutError("Backup health check timed out")


if __name__ == "__main__":
    if sys.argv[1:]: raise SystemExit("This fixed read-only check accepts no arguments")
    signal.signal(signal.SIGALRM, interrupted); signal.alarm(130)
    try:
        report = collect()
        problems = evaluate(report)
        signal.alarm(0)
        print(json.dumps(report), flush=True)
        raise SystemExit(1 if problems else 0)
    except Exception:
        print('{"error":"health_check_unavailable"}', flush=True)
        raise SystemExit(1)
