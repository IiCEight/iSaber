"""JD sender: coordinated snapshots, public-key encryption and immutable retries."""
import argparse
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import sys
import tarfile
import tempfile

sys.path.insert(0, str(Path(__file__).resolve().parent))
from backup import checksum, lock, snapshot, verify_snapshot
from staging import write_private, sync_directory
from vault_protocol import canonical, validate_id


def ssh_arguments(root):
    directory = root / "offsite"
    config = json.loads((directory / "config.json").read_text())
    # Host and port are root-owned operator configuration, never RPC input.
    return ["ssh", "-F", "/dev/null", "-i", str(directory / "upload_key"), "-o", "BatchMode=yes", "-o", "IdentitiesOnly=yes", "-o", "StrictHostKeyChecking=yes", "-o", "UserKnownHostsFile=" + str(directory / "known_hosts"), "-o", "ConnectTimeout=15", "-o", "ServerAliveInterval=15", "-o", "ServerAliveCountMax=3", "-p", str(config["port"]), "isaber-vault@" + config["host"], "store"]


def prepare(root, directory):
    metadata = verify_snapshot(root, directory)
    validate_id(directory.name)
    outbox = root / "offsite" / "outbox"
    outbox.mkdir(mode=0o700, exist_ok=True)
    target = outbox / directory.name
    if target.exists():
        if json.loads((target / "manifest.json").read_text())["backup_sha256"] != checksum(directory / "backup.json"):
            raise ValueError("Snapshot changed after encryption")
        return target
    config = json.loads((root / "offsite/config.json").read_text())
    source_bytes = sum(path.stat().st_size for path in directory.rglob("*") if path.is_file())
    if shutil.disk_usage(root).free < 5 * 1024**3 + 2 * source_bytes:
        raise OSError("Keep 5 GiB free on JD; encrypted copies were retained")
    with tempfile.TemporaryDirectory(prefix=".pending-", dir=outbox) as temporary:
        work = Path(temporary)
        plain = work / "snapshot.tar"
        with tarfile.open(plain, "w", format=tarfile.USTAR_FORMAT) as archive:
            for path in sorted(directory.rglob("*")):
                if path.is_file(): archive.add(path, arcname=path.relative_to(directory).as_posix(), recursive=False)
        plain.chmod(0o600)
        cipher = work / "snapshot.age"
        subprocess.run(["age", "-r", config["recipient"], "-o", str(cipher), str(plain)], check=True, capture_output=True, timeout=600)
        manifest = {"version": 1, "id": directory.name, "cipher_bytes": cipher.stat().st_size, "cipher_sha256": checksum(cipher), "plain_bytes": plain.stat().st_size, "plain_sha256": checksum(plain), "backup_sha256": checksum(directory / "backup.json"), "source_commit": metadata["release"]["source_commit"], "recipient": config["recipient"]}
        write_private(work / "manifest.json", canonical(manifest).decode())
        subprocess.run(["ssh-keygen", "-Y", "sign", "-f", str(root / "offsite/signing_key"), "-n", "isaber-backup", str(work / "manifest.json")], check=True, capture_output=True, timeout=15)
        (work / "manifest.json.sig").rename(work / "signature")
        plain.unlink()
        for path in work.iterdir():
            path.chmod(0o600)
            with path.open("rb") as source: os.fsync(source.fileno())
        sync_directory(work)
        work.rename(target)
        sync_directory(outbox)
    return target


def transfer(root, directory):
    manifest = json.loads((directory / "manifest.json").read_text())
    if checksum(directory / "snapshot.age") != manifest["cipher_sha256"]:
        raise ValueError("Local encrypted outbox integrity failed")
    # Verify first: after an interrupted acknowledgement a retry must not resend
    # bytes to an already committed immutable object.
    base = {"manifest": manifest, "signature": (directory / "signature").read_text()}
    probe = subprocess.run(ssh_arguments(root), input=(json.dumps({**base, "operation": "verify"}) + "\n").encode(), capture_output=True, timeout=90)
    if probe.returncode == 0:
        receipt = json.loads(probe.stdout)
    else:
        with tempfile.TemporaryFile() as response, tempfile.TemporaryFile() as errors:
            process = subprocess.Popen(ssh_arguments(root), stdin=subprocess.PIPE, stdout=response, stderr=errors)
            try:
                process.stdin.write((json.dumps({**base, "operation": "put"}) + "\n").encode())
                with (directory / "snapshot.age").open("rb") as source:
                    shutil.copyfileobj(source, process.stdin, 1024 * 1024)
                process.stdin.close()
                if process.wait(timeout=1600): raise RuntimeError("Encrypted vault upload failed")
                response.seek(0); receipt = json.load(response)
            finally:
                if process.poll() is None: process.kill(); process.wait()
    if receipt.get("status") not in ("stored", "exists") or receipt.get("manifest") != manifest:
        raise ValueError("Vault acknowledgement differs from the signed snapshot")
    write_private(directory / "receipt.json", json.dumps(receipt, indent=2) + "\n")
    status = json.dumps({"state": "uploaded", "snapshot_id": manifest["id"], "source_commit": manifest["source_commit"], "checked_at": datetime.now(timezone.utc).isoformat(), "cipher_sha256": manifest["cipher_sha256"]}, indent=2) + "\n"
    # Preserve the last successful capture when a later attempt fails. Freshness
    # is measured from its snapshot ID, never from a retry's acknowledgement.
    write_private(root / "offsite/last-success.json", status)
    write_private(root / "offsite/status.json", status)
    print("Encrypted offsite snapshot confirmed:", manifest["id"], flush=True)


def daily(root):
    with lock(root):
        pending = root / "offsite/pending.json"
        if pending.exists():
            identifier = json.loads(pending.read_text())["id"]
            validate_id(identifier)
            directory = root / "backups" / identifier
        else:
            if shutil.disk_usage(root).free < 5 * 1024**3:
                raise OSError("Keep 5 GiB free on JD before taking a snapshot")
            directory = snapshot(root)
            write_private(pending, json.dumps({"id": directory.name}) + "\n")
        transfer(root, prepare(root, directory))
        pending.unlink()
        sync_directory(pending.parent)


def interrupted(*_):
    # Raise through snapshot's finally block so systemd stop/timeout resumes
    # paused blog services before the unit exits.
    raise TimeoutError("Offsite backup interrupted")


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", type=Path, default=Path("/opt/isaber-staging"))
    options = parser.parse_args()
    os.umask(0o077)
    for signum in (signal.SIGTERM, signal.SIGINT, signal.SIGHUP):
        signal.signal(signum, interrupted)
    try:
        daily(options.root)
    except Exception as error:
        write_private(options.root / "offsite/status.json", json.dumps({"state": "failed", "error_type": type(error).__name__, "checked_at": datetime.now(timezone.utc).isoformat()}) + "\n")
        print("Offsite backup failed (" + type(error).__name__ + "); private output omitted", file=sys.stderr)
        raise SystemExit(1)
