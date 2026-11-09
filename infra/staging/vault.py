"""Append-only encrypted vault. The JD key cannot read, overwrite, or delete backups."""
import argparse
from datetime import datetime, timezone
import fcntl
import hashlib
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import tarfile
import tempfile
import time

sys.path.insert(0, str(Path(__file__).resolve().parent))
from vault_protocol import canonical, validate_id, validate_manifest, validate_signature

ROOT = Path("/var/lib/isaber-backup-vault")
NAMESPACE = ROOT / "jd-staging"
MIN_FREE = 5 * 1024**3
MAX_STORED = 10 * 1024**3


def digest(path):
    value = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            value.update(block)
    return value.hexdigest()


def sync_directory(path):
    descriptor = os.open(path, os.O_RDONLY | os.O_DIRECTORY)
    try: os.fsync(descriptor)
    finally: os.close(descriptor)


def check_signature(manifest, signature, temporary):
    path = temporary / ".validation-signature"
    path.write_text(validate_signature(signature))
    result = subprocess.run(["ssh-keygen", "-Y", "verify", "-f", str(ROOT / "allowed_signers"), "-I", "isaber-jd-staging", "-n", "isaber-backup", "-s", str(path)], input=canonical(manifest), capture_output=True, timeout=15)
    if result.returncode:
        raise ValueError("Sender signature rejected")
    path.unlink()
    config = json.loads((ROOT / "policy.json").read_text())
    if manifest["recipient"] != config["recipient"]:
        raise ValueError("Unexpected recovery recipient")


def store(header, source, temporary):
    manifest = validate_manifest(header.get("manifest"))
    signature = validate_signature(header.get("signature"))
    check_signature(manifest, signature, temporary)
    target = NAMESPACE / manifest["id"]
    if target.exists():
        previous = json.loads((target / "manifest.json").read_text())
        if previous != manifest or digest(target / "snapshot.age") != manifest["cipher_sha256"]:
            raise ValueError("Existing immutable snapshot differs; overwrite is forbidden")
        return {"status": "exists", "manifest": previous, "received_at": json.loads((target / "receipt.json").read_text())["received_at"]}
    if header.get("operation") != "put":
        raise FileNotFoundError("Requested offsite snapshot is missing")
    import shutil
    used = sum(path.stat().st_size for path in NAMESPACE.glob("*/snapshot.age"))
    if used + manifest["cipher_bytes"] > MAX_STORED or shutil.disk_usage(ROOT).free < manifest["cipher_bytes"] + MIN_FREE:
        raise OSError("Vault capacity limit; existing backups were retained")
    started = time.monotonic()
    checksum = hashlib.sha256()
    remaining = manifest["cipher_bytes"]
    ciphertext = temporary / "snapshot.age"
    with ciphertext.open("xb") as output:
        while remaining:
            if time.monotonic() - started > 1500:
                raise TimeoutError("Vault transfer deadline exceeded")
            signal.alarm(45)
            block = source.read(min(remaining, 1024 * 1024))
            if not block: raise ValueError("Incomplete encrypted backup")
            if remaining == manifest["cipher_bytes"] and not block.startswith(b"age-encryption.org/v1\n"):
                raise ValueError("Only age-encrypted data may enter the vault")
            output.write(block); checksum.update(block); remaining -= len(block)
        output.flush(); os.fsync(output.fileno())
    signal.alarm(60)
    if checksum.hexdigest() != manifest["cipher_sha256"]:
        raise ValueError("Encrypted transfer integrity failed")
    record = {"status": "stored", "manifest": manifest, "received_at": datetime.now(timezone.utc).isoformat()}
    for name, data in (("manifest.json", canonical(manifest)), ("signature", signature.encode()), ("receipt.json", (json.dumps(record, indent=2) + "\n").encode())):
        with (temporary / name).open("xb") as output:
            output.write(data); output.flush(); os.fsync(output.fileno())
    (temporary / "signature").chmod(0o600)
    sync_directory(temporary)
    # The namespace lock serializes writers; an existing destination never enters
    # this path and may not be overwritten by a retry or a compromised uploader.
    temporary.rename(target)
    sync_directory(NAMESPACE)
    return record


def receive():
    os.umask(0o077)
    NAMESPACE.mkdir(mode=0o700, parents=True, exist_ok=True)
    with (ROOT / ".lock").open("a") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        raw = sys.stdin.buffer.readline(16385)
        if len(raw) > 16384 or not raw.endswith(b"\n"): raise ValueError("Invalid vault header")
        header = json.loads(raw)
        if not isinstance(header, dict) or header.get("operation") not in ("put", "verify"):
            raise ValueError("The upload key has no read/delete/shell operation")
        with tempfile.TemporaryDirectory(prefix=".incoming-", dir=NAMESPACE) as directory:
            return store(header, sys.stdin.buffer, Path(directory))


def export(snapshot_id):
    # This mode is operator-only. The forced SSH key can invoke only 'receive'.
    validate_id(snapshot_id)
    directory = NAMESPACE / snapshot_id
    with tarfile.open(fileobj=sys.stdout.buffer, mode="w|", format=tarfile.USTAR_FORMAT) as archive:
        for name in ("manifest.json", "signature", "snapshot.age"):
            path = directory / name
            if path.is_symlink() or not path.is_file(): raise ValueError("Invalid vault object")
            info = archive.gettarinfo(str(path), arcname=name)
            info.mtime = info.uid = info.gid = 0; info.uname = info.gname = ""
            with path.open("rb") as source: archive.addfile(info, source)


def interrupted(*_):
    raise TimeoutError("Vault operation interrupted")


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("operation", choices=("receive", "export"))
    parser.add_argument("--id")
    args = parser.parse_args()
    signal.signal(signal.SIGALRM, interrupted); signal.signal(signal.SIGHUP, interrupted)
    signal.alarm(60)
    try:
        if args.operation == "export":
            signal.alarm(1500); export(args.id)
        else:
            result = receive(); signal.alarm(0)
            print(json.dumps(result), flush=True)
    except Exception as error:
        if args.operation == "receive": print(json.dumps({"status": "failed", "error_type": type(error).__name__}), flush=True)
        else: print("Vault export failed; no private material was logged", file=sys.stderr)
        raise SystemExit(1)
