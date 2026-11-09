"""GitHub-hosted read-only SSH client; output contains only validated health data."""
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile

sys.path.insert(0, str(Path(__file__).resolve().parent))
from backup_health import evaluate, MESSAGES


def fetch(directory):
    key = directory / "identity"
    key.write_text(os.environ["BACKUP_MONITOR_SSH_KEY"].strip() + "\n"); key.chmod(0o600)
    public = os.environ["STAGING_SSH_HOST_KEY"].strip()
    host, port = os.environ["STAGING_HOST"], os.environ.get("STAGING_PORT", "22")
    if not re.fullmatch(r"ssh-ed25519 [A-Za-z0-9+/=]+(?: [^\r\n]*)?", public) or not re.fullmatch(r"[A-Za-z0-9.-]+", host) or not re.fullmatch(r"[0-9]{1,5}", port) or not 1 <= int(port) <= 65535:
        raise ValueError("Invalid pinned SSH destination")
    hosts = directory / "known_hosts"
    hosts.write_text("isaber-jd-monitor " + " ".join(public.split()[:2]) + "\n")
    arguments = ["ssh", "-F", "/dev/null", "-i", str(key), "-o", "IdentitiesOnly=yes", "-o", "BatchMode=yes", "-o", "ForwardAgent=no", "-o", "ClearAllForwardings=yes", "-o", "StrictHostKeyChecking=yes", "-o", "HostKeyAlias=isaber-jd-monitor", "-o", "UserKnownHostsFile=" + str(hosts), "-o", "ConnectTimeout=20", "-o", "ServerAliveInterval=15", "-o", "ServerAliveCountMax=3", "-p", port, "isaber-monitor@" + host, "backup-health"]
    with tempfile.TemporaryFile() as output, tempfile.TemporaryFile() as errors:
        result = subprocess.run(arguments, stdin=subprocess.DEVNULL, stdout=output, stderr=errors, timeout=180)
        if result.returncode not in (0, 1): raise RuntimeError("Cannot reach the restricted backup monitor")
        output.seek(0); raw = output.read(16385)
        if not raw or len(raw) > 16384: raise ValueError("Invalid bounded health response")
        return json.loads(raw)


def publish(report, now=None):
    now = now or datetime.now(timezone.utc)
    problems = evaluate(report, now)
    snapshot = report["last_snapshot_id"] or "none"
    summary = "## Offsite backup health\n\n" + ("FAILED" if problems else "Healthy") + "\n\n"
    summary += f"Last captured snapshot: `{snapshot}`\n\n"
    summary += "JD free space: " + f'{report["disk_free_bytes"] / 1024**3:.1f} GiB' + "\n\n"
    if problems:
        for code in problems:
            print("::error title=Offsite backup: " + code + "::" + MESSAGES[code], flush=True)
            summary += "- " + MESSAGES[code] + "\n"
    else:
        print("Backup healthy: timer enabled, capture within 26 hours, Tencent ciphertext verified", flush=True)
    summary += "\nSee `docs/operations/backup-alerts.md` for diagnosis and recovery commands.\n"
    if os.environ.get("GITHUB_STEP_SUMMARY"):
        with Path(os.environ["GITHUB_STEP_SUMMARY"]).open("a") as output: output.write(summary)
    return 1 if problems else 0


if __name__ == "__main__":
    try:
        with tempfile.TemporaryDirectory(prefix="isaber-backup-monitor-") as temporary:
            raise SystemExit(publish(fetch(Path(temporary))))
    except Exception:
        print("::error title=Offsite backup monitor unavailable::Cannot obtain a valid backup health report. Check SSH reachability, monitor credentials, and installed monitor code; no backup was changed.", flush=True)
        if os.environ.get("GITHUB_STEP_SUMMARY"):
            with Path(os.environ["GITHUB_STEP_SUMMARY"]).open("a") as output:
                output.write("## Offsite backup health\n\nFAILED: the external monitor could not obtain a valid report. Inspect the restricted SSH monitor and server reachability. No backup was changed.\n")
        raise SystemExit(1)
