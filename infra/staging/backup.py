"""Coordinated private staging snapshots and disposable DB/media restore checks."""
import argparse
from contextlib import contextmanager
from datetime import datetime, timezone
import fcntl
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import secrets
import shutil
import subprocess
import tarfile
import tempfile

from staging import CONFIG, PROJECT, compose, compose_arguments, credentials, run, sql, validate_release, verify_https, write_private, sync_directory


def checksum(path):
    value = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            value.update(block)
    return value.hexdigest()


def stream(arguments, source=None, destination=None):
    result = subprocess.run(arguments, stdin=source, stdout=destination if destination is not None else subprocess.PIPE, stderr=subprocess.PIPE, timeout=600)
    if result.returncode:
        raise RuntimeError("Snapshot stream failed; raw output omitted to protect credentials")


def tar_inventory(path):
    files = {}
    names = set()
    with tarfile.open(path, "r|*") as archive:
        for member in archive:
            name = member.name.removeprefix("./")
            parsed = PurePosixPath(name)
            if parsed.is_absolute() or ".." in parsed.parts or "\\" in name or name in names or not (member.isfile() or member.isdir()):
                raise ValueError("Unsafe media archive entry")
            names.add(name)
            if member.isfile():
                value = hashlib.sha256()
                with archive.extractfile(member) as content:
                    for block in iter(lambda: content.read(1024 * 1024), b""):
                        value.update(block)
                files[name] = {"size": member.size, "sha256": value.hexdigest()}
    return files


@contextmanager
def lock(root):
    credentials(root)
    with (root / ".backup.lock").open("a") as handle:
        os.chmod(handle.name, 0o600)
        fcntl.flock(handle, fcntl.LOCK_EX | fcntl.LOCK_NB)
        yield


def row_counts(root, database="isaber_staging"):
    def query(statement):
        return compose(root, "exec", "-T", "postgres", "psql", "-X", "-v", "ON_ERROR_STOP=1", "-U", "isaber_operator", "-d", database, "-At", data=statement.encode())
    tables = query("SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename;").decode().splitlines()
    result = {}
    for table in tables:
        if not re.fullmatch(r"[a-z_0-9]+", table):
            raise ValueError("Unexpected staging table name")
        result[table] = int(query(f'SELECT count(*) FROM "{table}";').strip())
    return result


def table_hashes(root, database, tables):
    result = {}
    for table in tables:
        if not re.fullmatch(r"[a-z_0-9]+", table):
            raise ValueError("Unexpected staging table name")
        query = f'''SET statement_timeout='120s'; COPY (SELECT row_to_json(t)::text FROM "{table}" AS t ORDER BY row_to_json(t)::text COLLATE "C") TO STDOUT;'''
        with tempfile.TemporaryFile() as output:
            stream(compose_arguments(root, "exec", "-T", "postgres", "psql", "-X", "-v", "ON_ERROR_STOP=1", "-U", "isaber_operator", "-d", database, "-qAt", "-c", query), destination=output)
            output.seek(0)
            value = hashlib.sha256()
            for block in iter(lambda: output.read(1024 * 1024), b""):
                value.update(block)
            result[table] = value.hexdigest()
    return result


def volume_command(image, volume, target, *arguments, writable=False, user="65532:65532"):
    mount = f"type=volume,source={volume},target={target}" + ("" if writable else ",readonly")
    return ["docker", "run", "--rm", "-i", "--pull=never", "--network=none", "--read-only", "--cap-drop=ALL", "--security-opt=no-new-privileges", "--user", user, "--mount", mount, "--entrypoint", arguments[0], image, *arguments[1:]]


def snapshot(root):
    if (root / "pending-promotion.json").exists():
        raise ValueError("Recover the pending promotion before taking another snapshot")
    release = json.loads((root / "current-release.json").read_text())
    validate_release(release)
    image = release["images"]["postgres"]["reference"]
    backups = root / "backups"
    backups.mkdir(mode=0o700, exist_ok=True)
    name = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ-") + secrets.token_hex(4)
    pending = backups / (".pending-" + name)
    pending.mkdir(mode=0o700)
    running = set(compose(root, "ps", "--services", "--status", "running").decode().split())
    if "postgres" not in running:
        shutil.rmtree(pending)
        raise ValueError("Staging database must be running for its logical snapshot")
    paused = [service for service in ("api", "frontend", "edge") if service in running]
    try:
        print("Pausing staging writers for a coordinated database/media snapshot", flush=True)
        if paused:
            compose(root, "stop", *reversed(paused))
        counts = row_counts(root)
        hashes = table_hashes(root, "isaber_staging", counts)
        with (pending / "database.pgdump").open("xb") as output:
            os.chmod(output.name, 0o600)
            stream(compose_arguments(root, "exec", "-T", "postgres", "pg_dump", "-U", "isaber_operator", "-d", "isaber_staging", "--format=custom", "--no-owner", "--no-acl"), destination=output)
        for label, volume, user in (("uploads", PROJECT + "_uploads", "65532:65532"), ("certificates", PROJECT + "_certificates", "0:0")):
            with (pending / (label + ".tar")).open("xb") as output:
                os.chmod(output.name, 0o600)
                stream(volume_command(image, volume, "/source", "tar", "--numeric-owner", "-C", "/source", "-cf", "-", ".", user=user), destination=output)
            tar_inventory(pending / (label + ".tar"))
        for relative in ("secrets.env", "release.env", "current-release.json", "staging-root.crt", "publish_images.py"):
            shutil.copyfile(root / relative, pending / relative)
            (pending / relative).chmod(0o600)
        (pending / "config").mkdir(mode=0o700)
        for path in CONFIG.iterdir():
            if path.is_file() and not path.is_symlink():
                shutil.copyfile(path, pending / "config" / path.name)
                (pending / "config" / path.name).chmod(0o600)
        inventory = {path.relative_to(pending).as_posix(): {"size": path.stat().st_size, "sha256": checksum(path)} for path in pending.rglob("*") if path.is_file()}
        for relative in inventory:
            with (pending / relative).open("rb") as source:
                os.fsync(source.fileno())
        sync_directory(pending / "config")
        write_private(pending / "backup.json", json.dumps({"format_version": 1, "release": release, "row_counts": counts, "table_hashes": hashes, "uploads": tar_inventory(pending / "uploads.tar"), "files": inventory}, indent=2) + "\n")
        destination = backups / name
        pending.rename(destination)
        sync_directory(backups)
        print("Completed private staging snapshot:", destination, flush=True)
        return destination
    finally:
        if pending.exists():
            shutil.rmtree(pending)
        if paused:
            compose(root, "up", "-d", "--wait", *paused)
            if "edge" in paused:
                verify_https(root)


def verify_snapshot(root, directory):
    # Only select operator-created snapshots inside the dedicated private tree.
    if directory.is_symlink() or directory.resolve().parent != (root / "backups").resolve():
        raise ValueError("Select a snapshot directly under the staging backups directory")
    metadata = json.loads((directory / "backup.json").read_text())
    if metadata.get("format_version") != 1:
        raise ValueError("Unsupported staging snapshot")
    validate_release(metadata["release"])
    actual = set()
    for path in directory.rglob("*"):
        if path.is_symlink():
            raise ValueError("Snapshot symlinks are forbidden")
        if path.is_file():
            actual.add(path.relative_to(directory).as_posix())
    if actual != set(metadata["files"]) | {"backup.json", "restore-proof.json"} and actual != set(metadata["files"]) | {"backup.json"}:
        raise ValueError("Snapshot file inventory mismatch")
    for relative, record in metadata["files"].items():
        parsed = PurePosixPath(relative)
        if parsed.is_absolute() or ".." in parsed.parts or "\\" in relative or parsed.as_posix() != relative:
            raise ValueError("Unsafe snapshot path")
        path = directory / relative
        if path.stat().st_size != record["size"] or checksum(path) != record["sha256"]:
            raise ValueError("Snapshot integrity check failed")
    if tar_inventory(directory / "uploads.tar") != metadata["uploads"]:
        raise ValueError("Snapshot media inventory mismatch")
    tar_inventory(directory / "certificates.tar")
    return metadata


def restore_check(root, directory):
    metadata = verify_snapshot(root, directory)
    image = metadata["release"]["images"]["postgres"]["reference"]
    token = secrets.token_hex(8)
    database = "isaber_restore_" + token
    volume = "isaber-restore-" + token
    db_created = False
    volume_created = False
    try:
        sql(root, f'CREATE DATABASE "{database}";')
        db_created = True
        with (directory / "database.pgdump").open("rb") as source:
            stream(compose_arguments(root, "exec", "-T", "postgres", "pg_restore", "-U", "isaber_operator", "-d", database, "--no-owner", "--no-privileges", "--exit-on-error"), source=source)
        if row_counts(root, database) != metadata["row_counts"]:
            raise ValueError("Restored database row counts differ from the snapshot")
        if table_hashes(root, database, metadata["row_counts"]) != metadata["table_hashes"]:
            raise ValueError("Restored database contents differ from the snapshot")
        run(["docker", "volume", "create", volume])
        volume_created = True
        # Own only the newly created disposable volume, never a live volume.
        run(["docker", "run", "--rm", "--pull=never", "--network=none", "--read-only", "--cap-drop=ALL", "--cap-add=CHOWN", "--security-opt=no-new-privileges", "--mount", f"type=volume,source={volume},target=/restore", "--entrypoint=chown", image, "65532:65532", "/restore"])
        with (directory / "uploads.tar").open("rb") as source:
            stream(volume_command(image, volume, "/restore", "tar", "--no-same-owner", "-C", "/restore", "-xf", "-", writable=True), source=source)
        temporary = directory / ".restored-media.tar"
        try:
            with temporary.open("xb") as output:
                temporary.chmod(0o600)
                stream(volume_command(image, volume, "/source", "tar", "--numeric-owner", "-C", "/source", "-cf", "-", "."), destination=output)
            if tar_inventory(temporary) != metadata["uploads"]:
                raise ValueError("Restored media bytes differ from the snapshot")
        finally:
            temporary.unlink(missing_ok=True)
        write_private(directory / "restore-proof.json", json.dumps({"checked_at": datetime.now(timezone.utc).isoformat(), "backup_sha256": checksum(directory / "backup.json"), "database_rows_match": True, "database_contents_match": True, "media_bytes_match": True}, indent=2) + "\n")
        print("Disposable restore passed: database rows and media bytes match; live database/media were not overwritten", flush=True)
    finally:
        if db_created:
            sql(root, f'DROP DATABASE "{database}" WITH (FORCE);')
        if volume_created:
            run(["docker", "volume", "rm", volume])


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("command", choices=("snapshot", "restore-check"))
    parser.add_argument("--root", type=Path, default=Path("/opt/isaber-staging"))
    parser.add_argument("--snapshot", type=Path)
    options = parser.parse_args()
    try:
        with lock(options.root):
            if options.command == "snapshot":
                snapshot(options.root)
            else:
                if not options.snapshot:
                    raise ValueError("Select a --snapshot directory")
                restore_check(options.root, options.snapshot)
    except Exception as error:
        print(f"Staging backup operation failed ({type(error).__name__}); raw secret-bearing output omitted")
        raise SystemExit(1)
