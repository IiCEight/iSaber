"""Receive a decrypted, signed-digest-bound archive; test only isolated resources."""
import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import secrets
import shutil
import subprocess
import sys
import tarfile
import time

sys.path.insert(0, str(Path(__file__).resolve().parent))
from backup import checksum, lock, row_counts, stream, table_hashes, tar_inventory, verify_snapshot, volume_command
from staging import run, write_private
from vault_protocol import validate_manifest


def extract_plain(source, destination, manifest):
    """Parse raw USTAR headers: reject extensions/links before tarfile expands them."""
    remaining = manifest["plain_bytes"]
    checksum_state = hashlib.sha256()
    names = set()

    def read(size):
        nonlocal remaining
        if size > remaining: raise ValueError("Archive exceeds signed length")
        data = source.read(size)
        if len(data) != size: raise ValueError("Truncated archive")
        checksum_state.update(data); remaining -= size
        return data

    while remaining:
        header = read(512)
        if header == bytes(512):
            while remaining:
                if any(read(min(remaining, 1024 * 1024))): raise ValueError("Unexpected trailing archive data")
            break
        member = tarfile.TarInfo.frombuf(header, "utf-8", "strict")
        parsed = PurePosixPath(member.name)
        if not member.isfile() or parsed.is_absolute() or ".." in parsed.parts or "\\" in member.name or parsed.as_posix() != member.name or member.name in names or member.size < 0:
            raise ValueError("Unsafe or duplicate archive member")
        names.add(member.name)
        if len(names) > 256: raise ValueError("Too many snapshot files")
        path = destination / member.name
        path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
        with path.open("xb") as output:
            path.chmod(0o600)
            size = member.size
            while size:
                block = read(min(size, 1024 * 1024)); output.write(block); size -= len(block)
        read((-member.size) % 512)
    if checksum_state.hexdigest() != manifest["plain_sha256"] or checksum(destination / "backup.json") != manifest["backup_sha256"]:
        raise ValueError("Decrypted snapshot differs from signed checksums")


def isolated_check(root, directory):
    metadata = verify_snapshot(root, directory)
    # Select the locally installed trusted image, not an image named by the vault.
    image = json.loads((root / "current-release.json").read_text())["images"]["postgres"]["reference"]
    token = secrets.token_hex(8)
    container = "isaber-offsite-check-" + token
    dbvolume, media = container + "-db", container + "-media"
    created = []
    try:
        for volume, owner in ((dbvolume, "999:999"), (media, "65532:65532")):
            run(["docker", "volume", "create", volume]); created.append(volume)
            run(["docker", "run", "--rm", "--pull=never", "--network=none", "--read-only", "--cap-drop=ALL", "--cap-add=CHOWN", "--security-opt=no-new-privileges", "--mount", f"type=volume,source={volume},target=/restore", "--entrypoint=chown", image, owner, "/restore"])
        run(["docker", "run", "-d", "--name", container, "--pull=never", "--network=none", "--read-only", "--cap-drop=ALL", "--security-opt=no-new-privileges", "--user", "999:999", "--memory=256m", "--cpus=0.5", "--pids-limit=128", "--tmpfs", "/tmp:rw,noexec,nosuid,size=64m", "--tmpfs", "/var/run/postgresql:rw,noexec,nosuid,size=8m,uid=999,gid=999", "--mount", f"type=volume,source={dbvolume},target=/var/lib/postgresql/data", "-e", "POSTGRES_HOST_AUTH_METHOD=trust", "-e", "POSTGRES_USER=backup_check", "-e", "POSTGRES_DB=backup_check", image, "postgres", "-c", "shared_buffers=32MB", "-c", "max_connections=10"])
        psql = ["docker", "exec", "-i", container, "psql", "-X", "-v", "ON_ERROR_STOP=1", "-U", "backup_check", "-d", "backup_check"]
        for attempt in range(60):
            try:
                run([*psql, "-At", "-c", "SELECT 1;"]); break
            except RuntimeError:
                if attempt == 59: raise
                time.sleep(1)
        with (directory / "database.pgdump").open("rb") as source:
            stream(["docker", "exec", "-i", container, "pg_restore", "-U", "backup_check", "-d", "backup_check", "--no-owner", "--no-privileges", "--exit-on-error"], source=source)
        counts = row_counts(root, query=lambda statement: run([*psql, "-At"], data=statement.encode()))
        if counts != metadata["row_counts"] or table_hashes(root, "backup_check", counts, psql=psql) != metadata["table_hashes"]:
            raise ValueError("Restored database contents differ")
        with (directory / "uploads.tar").open("rb") as source:
            stream(volume_command(image, media, "/restore", "tar", "--no-same-owner", "-C", "/restore", "-xf", "-", writable=True), source=source)
        media_tar = directory / ".media-check.tar"
        try:
            with media_tar.open("xb") as output:
                stream(volume_command(image, media, "/restore", "tar", "-C", "/restore", "-cf", "-", "."), destination=output)
            if tar_inventory(media_tar) != metadata["uploads"]: raise ValueError("Restored media bytes differ")
        finally: media_tar.unlink(missing_ok=True)
        proof = {"checked_at": datetime.now(timezone.utc).isoformat(), "backup_sha256": checksum(directory / "backup.json"), "database_rows_match": True, "database_contents_match": True, "media_bytes_match": True, "isolated_container": True}
        write_private(root / "offsite/restore-proof.json", json.dumps(proof, indent=2) + "\n")
        print(json.dumps(proof), flush=True)
    finally:
        # A failed docker run can leave a created-but-unstarted container.
        # This random name belongs solely to this check, even on startup errors.
        subprocess.run(["docker", "rm", "-f", container], capture_output=True, timeout=60)
        for volume in reversed(created): run(["docker", "volume", "rm", volume])


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", type=Path, default=Path("/opt/isaber-staging"))
    args = parser.parse_args()
    os.umask(0o077)
    directory = None
    try:
        manifest = validate_manifest(json.loads(sys.stdin.buffer.readline(16385)))
        with lock(args.root):
            directory = args.root / "backups" / ("retrieved-" + secrets.token_hex(8))
            directory.mkdir(mode=0o700)
            extract_plain(sys.stdin.buffer, directory, manifest)
            isolated_check(args.root, directory)
    except Exception as error:
        print("Offsite restore check failed (" + type(error).__name__ + "); private output omitted", file=sys.stderr)
        raise SystemExit(1)
    finally:
        if directory is not None: shutil.rmtree(directory)
