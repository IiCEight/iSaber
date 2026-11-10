"""Fixed privileged receiver: verified main artifacts, never uploaded program execution."""
import contextlib
import fcntl
import hashlib
import io
import json
import os
from pathlib import Path
import re
import signal
import shutil
import sys
import tempfile
import tarfile
import time
import urllib.request
import urllib.error
import zipfile
from pathlib import PurePosixPath

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))
sys.path.insert(0, str(Path(__file__).resolve().parent.parent / "config"))
sys.path.insert(0, str(Path(__file__).resolve().parent))
from relay_release import check_run, REPOSITORY, NoRedirect
from receive_artifact import StorageRedirect, validate_url
from release_bundle import verify, publish, validate_header, METADATA_FILES, BASELINE_FILE
from staging import compatible_release, promote, write_private, verify_https as check_staging_https, run as installed_run

ROOT = Path("/opt/isaber-staging")
MAX_BYTES = 512 * 1024**2
PHASE = "header"
TRANSFER_IDLE_SECONDS = 120
TRANSFER_DEADLINE_SECONDS = 1800
TRANSFER_BYTES = 0


class StaleRelease(Exception):
    pass


class GitHub:
    def __init__(self, token):
        if not isinstance(token, str) or not re.fullmatch(r"[A-Za-z0-9_.-]{10,2048}", token):
            raise ValueError("A temporary read-only job token is required")
        self.token = token

    def get(self, path):
        request = urllib.request.Request(f"https://api.github.com/repos/{REPOSITORY}/" + path, headers={"Authorization": "Bearer " + self.token, "Accept": "application/vnd.github+json"})
        with urllib.request.build_opener(NoRedirect).open(request, timeout=20) as response:
            data = response.read(2 * 1024**2 + 1)
        if len(data) > 2 * 1024**2:
            raise ValueError("GitHub metadata exceeds the limit")
        return json.loads(data)

    def metadata_archive(self, artifact):
        request = urllib.request.Request(f"https://api.github.com/repos/{REPOSITORY}/actions/artifacts/{int(artifact['id'])}/zip", headers={"Authorization": "Bearer " + self.token})
        try:
            urllib.request.build_opener(NoRedirect).open(request, timeout=20)
            raise ValueError("Expected metadata artifact redirect")
        except urllib.error.HTTPError as error:
            if error.code != 302:
                raise RuntimeError("Metadata redirect failed") from None
            url = error.headers.get("Location", "")
            error.close()
        validate_url(url)
        # The job token is sent only to api.github.com, never the storage URL.
        with urllib.request.build_opener(StorageRedirect).open(url, timeout=30) as response:
            return response.read(2 * 1024**2 + 1)


def select_artifact(run, artifacts):
    check_run(run, None, None)
    attempt = run.get("run_attempt")
    if type(attempt) is not int or attempt < 1:
        raise ValueError("Invalid source attempt")
    matches = [item for item in artifacts if not item.get("expired") and re.fullmatch(r"bundle-[0-9a-f]{40}-" + str(attempt), item.get("name", ""))]
    if len(matches) != 1:
        raise ValueError("Expected one unexpired source bundle")
    artifact = matches[0]
    if not re.fullmatch(r"sha256:[0-9a-f]{64}", artifact.get("digest", "")) or type(artifact.get("size_in_bytes")) is not int or not 0 < artifact["size_in_bytes"] <= MAX_BYTES:
        raise ValueError("Source artifact lacks a bounded integrity receipt")
    return artifact


def require_current(api, commit):
    if api.get("git/ref/heads/main")["object"]["sha"] != commit:
        raise StaleRelease()


def receive(stream, path, size, expected, offset=0):
    global TRANSFER_BYTES
    started = time.monotonic()
    value = hashlib.sha256()
    remaining = size - offset
    TRANSFER_BYTES = offset
    if offset:
        if path.is_symlink() or path.stat().st_size != offset:
            raise ValueError("Partial transfer changed; request a fresh plan")
        with path.open("rb") as previous:
            for block in iter(lambda: previous.read(1024 * 1024), b""):
                value.update(block)
    # BufferedReader.read(n) can wait for all n bytes. read1 returns after one
    # underlying read, so the idle timer resets whenever the wire makes progress.
    read = getattr(stream, "read1", stream.read)
    with path.open("ab" if offset else "xb") as output:
        path.chmod(0o600)
        while remaining:
            available = TRANSFER_DEADLINE_SECONDS - (time.monotonic() - started)
            if available <= 0:
                raise TimeoutError("Transfer deadline exceeded")
            signal.setitimer(signal.ITIMER_REAL, min(TRANSFER_IDLE_SECONDS, available))
            block = read(min(64 * 1024, remaining))
            if not block:
                raise ValueError("Incomplete transfer")
            output.write(block)
            value.update(block)
            remaining -= len(block)
            TRANSFER_BYTES += len(block)
    signal.alarm(180)
    if "sha256:" + value.hexdigest() != expected:
        raise ValueError("Transfer checksum differs from GitHub receipt")
    return round(time.monotonic() - started, 2)


def expected_release(bundle):
    release = {key: bundle[key] for key in ("source_commit", "ci_run_id", "platform", "migration_version", "contract_sha256", "bundle_run_id", "bundle_run_attempt")}
    release.update(format_version=1, images={})
    for name, image in bundle["images"].items():
        release["images"][name] = {"reference": f"ccr.ccs.tencentyun.com/isaber/{name}@{image['manifest_digest']}", "image_id": image["image_id"]}
        if name == "postgres":
            release["images"][name]["upstream_reference"] = image["upstream_reference"]
    return release


def metadata_from_archive(raw, artifact):
    if len(raw) > 2 * 1024**2 or len(raw) != artifact.get("size_in_bytes") or "sha256:" + hashlib.sha256(raw).hexdigest() != artifact.get("digest"):
        raise ValueError("Metadata ZIP differs from GitHub receipt")
    with zipfile.ZipFile(io.BytesIO(raw)) as archive:
        if archive.namelist() != ["bundle.json"] or archive.getinfo("bundle.json").file_size > 2 * 1024**2:
            raise ValueError("Expected one bounded metadata file")
        return json.loads(archive.read("bundle.json"))


def load_source(header):
    global PHASE
    PHASE = "source_metadata"
    run_id = header.get("run_id")
    if type(run_id) is not int or run_id < 1:
        raise ValueError("A source preparation run is required")
    api = GitHub(header.get("token"))
    run = api.get(f"actions/runs/{run_id}")
    artifacts = api.get(f"actions/runs/{run_id}/artifacts?per_page=100")["artifacts"]
    artifact = select_artifact(run, artifacts)
    commit = artifact["name"].split("-")[1]
    require_current(api, commit)
    matches = [item for item in artifacts if not item.get("expired") and item.get("name") == f"metadata-{commit}-{run['run_attempt']}"]
    if len(matches) != 1:
        raise ValueError("Expected the attempt's independently authenticated metadata artifact")
    bundle = metadata_from_archive(api.metadata_archive(matches[0]), matches[0])
    validate_header(bundle, commit)
    attempt = run["run_attempt"]
    if bundle["format_version"] != 2 or str(bundle["bundle_run_id"]) != str(run_id) or str(bundle["bundle_run_attempt"]) != str(attempt):
        raise ValueError("Bundle is not the expected optimized workflow attempt")
    jobs = api.get(f"actions/runs/{run_id}/attempts/{attempt}/jobs?per_page=100")["jobs"]
    check_run(run, jobs, api.get("actions/runs/" + str(bundle["ci_run_id"])), commit)
    validate_inventory(bundle)
    return api, bundle


def validate_inventory(bundle):
    files = bundle.get("files")
    if not isinstance(files, dict) or len(files) > 1000 or not set((*METADATA_FILES, BASELINE_FILE)).issubset(files):
        raise ValueError("Invalid authenticated inventory")
    total = 0
    for relative, record in files.items():
        path = PurePosixPath(relative)
        if path.is_absolute() or ".." in path.parts or "\\" in relative or path.as_posix() != relative or (relative not in (*METADATA_FILES, BASELINE_FILE) and not re.fullmatch(r"images/(frontend|backend)/(manifest\.json|version|[0-9a-f]{64})", relative)):
            raise ValueError("Unsafe inventory path")
        if not isinstance(record, dict) or type(record.get("size")) is not int or record["size"] < 0 or not re.fullmatch(r"[0-9a-f]{64}", record.get("sha256", "")):
            raise ValueError("Invalid file receipt")
        total += record["size"]
    if total > 4 * 1024**3:
        raise ValueError("Bundle exceeds the reconstruction limit")


def file_hash(path):
    value = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            value.update(block)
    return value.hexdigest()


def cached(record):
    path = ROOT / "delivery/blobs" / record["sha256"]
    return path.is_file() and not path.is_symlink() and path.stat().st_size == record["size"] and file_hash(path) == record["sha256"]


def cache_file(path):
    cache = ROOT / "delivery/blobs"
    cache.mkdir(mode=0o700, exist_ok=True)
    target = cache / file_hash(path)
    with tempfile.NamedTemporaryFile(dir=cache, delete=False) as output, path.open("rb") as source:
        temporary = Path(output.name)
        shutil.copyfileobj(source, output)
    temporary.chmod(0o444)
    os.replace(temporary, target)


def seed_cache():
    global PHASE
    PHASE = "cache_seed"
    marker = ROOT / "delivery/cache-seeded.json"
    if marker.exists():
        return
    current = json.loads((ROOT / "current-release.json").read_text())
    for name in ("frontend", "backend"):
        with tempfile.TemporaryDirectory(prefix="seed-", dir=ROOT / "delivery") as temporary:
            directory = Path(temporary)
            tag = "isaber-staging-cache-" + name + ":seed"
            installed_run(["docker", "tag", current["images"][name]["reference"], tag])
            installed_run(["docker", "image", "save", tag, "--platform", "linux/amd64", "--output", str(directory / "image.tar")])
            source = "docker-archive:" + str(directory / "image.tar")
            config = installed_run(["skopeo", "inspect", "--config", "--raw", source])
            if "sha256:" + hashlib.sha256(config).hexdigest() != current["images"][name]["image_id"]:
                raise ValueError("Cache seed differs from the installed image")
            installed_run(["skopeo", "copy", "--dest-compress", source, "dir:" + str(directory / "image")])
            for path in (directory / "image").iterdir():
                if path.is_file(): cache_file(path)
    write_private(marker, json.dumps({"seed_commit": current["source_commit"]}) + "\n")


def plan(header):
    if (ROOT / "pending-promotion.json").exists():
        raise ValueError("Recover the interrupted promotion before planning delivery")
    api, bundle = load_source(header)
    compatible_release(json.loads((ROOT / "current-release.json").read_text()), expected_release(bundle))
    seed_cache()
    require_current(api, bundle["source_commit"])
    missing = [relative for relative, record in bundle["files"].items() if not cached(record)]
    return {"status": "plan", "bundle": bundle, "missing": missing, "bytes_missing": sum(bundle["files"][name]["size"] for name in missing), "partial_transfer": partial_transfer(bundle)}


def partial_transfer(bundle):
    # One bounded root-owned partial, shared under the delivery lock. New trusted
    # sources replace it; it can never select a path or enter the verified cache.
    receipt, archive = ROOT / "delivery/transfer.json", ROOT / "delivery/transfer.tar"
    if receipt.is_symlink() or archive.is_symlink():
        raise ValueError("Partial transfer must use regular private files")
    if not receipt.exists() or not archive.exists():
        return None
    if receipt.stat().st_size > 2048 or not receipt.is_file() or not archive.is_file():
        raise ValueError("Invalid partial transfer receipt")
    record = json.loads(receipt.read_text())
    if any(str(record.get(key)) != str(bundle[key]) for key in ("source_commit", "bundle_run_id", "bundle_run_attempt")):
        return None
    size, offset = record.get("size"), archive.stat().st_size
    if type(size) is not int or not 0 <= offset <= size <= MAX_BYTES or not re.fullmatch(r"sha256:[0-9a-f]{64}", record.get("digest", "")):
        raise ValueError("Invalid partial transfer size or digest")
    return {"size": size, "digest": record["digest"], "offset": offset, "prefix_sha256": file_hash(archive)}


def reconstruct(archive, content, bundle):
    validate_inventory(bundle)
    content.mkdir(mode=0o700)
    names = set()
    with archive.open("rb") as source:
        while True:
            header = source.read(512)
            if header == b"\0" * 512:
                break
            if len(header) != 512:
                raise ValueError("Truncated thin archive")
            # Decode one fixed-size USTAR header without processing PAX/GNU
            # extension bodies, which could otherwise force large allocations.
            member = tarfile.TarInfo.frombuf(header, "utf-8", "strict")
            record = bundle["files"].get(member.name)
            if not member.isfile() or member.name in names or record is None or member.size != record["size"]:
                raise ValueError("Unexpected thin-archive entry")
            names.add(member.name)
            target = content / member.name
            target.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
            with target.open("xb") as output:
                remaining = member.size
                while remaining:
                    block = source.read(min(remaining, 1024 * 1024))
                    if not block:
                        raise ValueError("Truncated thin file")
                    output.write(block)
                    remaining -= len(block)
            padding = (512 - member.size % 512) % 512
            if source.read(padding) != b"\0" * padding:
                raise ValueError("Invalid thin-file padding")
            target.chmod(0o600)
            if file_hash(target) != record["sha256"]:
                raise ValueError("Thin file differs from authenticated GitHub inventory")
    for relative, record in bundle["files"].items():
        target = content / relative
        if target.exists():
            continue
        if not cached(record):
            raise ValueError("A planned cache file is missing; retry with a fresh plan")
        target.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
        os.link(ROOT / "delivery/blobs" / record["sha256"], target)
    (content / "bundle.json").write_text(json.dumps(bundle))
    verify(content, bundle["source_commit"])
    for relative in names:
        cache_file(content / relative)


def deliver(header, stream, directory, verify_only=False):
    global PHASE
    if (ROOT / "pending-promotion.json").exists():
        raise ValueError("Recover the interrupted promotion before delivery")
    api, bundle = load_source(header)
    size, expected = header.get("size"), header.get("digest", "")
    if type(size) is not int or not 0 < size <= MAX_BYTES or not re.fullmatch(r"sha256:[0-9a-f]{64}", expected):
        raise ValueError("Invalid bounded thin archive")
    archive = ROOT / "delivery/transfer.tar"
    receipt = ROOT / "delivery/transfer.json"
    previous = partial_transfer(bundle)
    offset = header.get("offset", 0)
    if type(offset) is not int or not 0 <= offset <= size:
        raise ValueError("Invalid transfer resume offset")
    if offset:
        if previous != {"size": size, "digest": expected, "offset": offset, "prefix_sha256": header.get("prefix_sha256")}:
            raise ValueError("Partial transfer changed; request a fresh plan")
    else:
        archive.unlink(missing_ok=True)
        record = {key: bundle[key] for key in ("source_commit", "bundle_run_id", "bundle_run_attempt")}
        write_private(receipt, json.dumps({**record, "size": size, "digest": expected}) + "\n")
    if shutil.disk_usage(ROOT).free < size * 3 + 1024**3:
        raise ValueError("Insufficient free delivery/cache disk")
    PHASE = "wire_transfer"
    seconds = receive(stream, archive, size, expected, offset)
    content = directory / "content"
    PHASE = "bundle_verification"
    reconstruct(archive, content, bundle)
    archive.unlink()
    receipt.unlink()
    # Uploaded programs remain hash-bound data. Only installed root-owned tools run.
    commit, run_id, attempt = bundle["source_commit"], bundle["bundle_run_id"], bundle["bundle_run_attempt"]
    candidate = expected_release(bundle)
    current = json.loads((ROOT / "current-release.json").read_text())
    compatible_release(current, candidate)
    require_current(api, commit)
    if verify_only:
        return {"status": "verified", "release": candidate, "current_commit": current["source_commit"], "transfer_seconds": seconds}
    if current == candidate:
        # A matching receipt alone does not prove the containers are healthy.
        # Keep the structured SSH response separate from operator check output.
        with io.StringIO() as checks, contextlib.redirect_stdout(checks):
            check_staging_https(ROOT)
        return {"status": "already_current", "release": current, "transfer_seconds": seconds}
    require_current(api, commit)
    signal.alarm(600)
    signal.signal(signal.SIGHUP, signal.SIG_IGN)
    logfile = ROOT / "delivery" / f"{run_id}-{attempt}.log"
    with logfile.open("a") as log, contextlib.redirect_stdout(log), contextlib.redirect_stderr(log):
        logfile.chmod(0o600)
        output = directory / "release.json"
        PHASE = "registry_publish"
        publish(content, output, commit, "/root/.docker/config.json")
        if json.loads(output.read_text()) != candidate:
            raise ValueError("Publisher metadata differs from the tested bundle")
        require_current(api, commit)
        PHASE = "staging_promotion"
        promote(ROOT, output, before_switch=lambda: require_current(api, commit))
    record = {"status": "deployed", "release": candidate, "transfer_seconds": seconds}
    write_private(ROOT / "delivery" / f"{run_id}-{attempt}.json", json.dumps(record, indent=2) + "\n")
    return record


def handle(header, stream, directory, probe_only=False):
    operation = header.get("operation")
    if operation == "probe":
        size = header.get("size")
        expected = header.get("digest", "")
        if type(size) is not int or not 0 < size <= 8 * 1024**2 or not re.fullmatch(r"sha256:[0-9a-f]{64}", expected):
            raise ValueError("Invalid bounded probe")
        seconds = receive(stream, directory / "probe", size, expected)
        return {"status": "probe", "bytes": size, "transfer_seconds": seconds}
    if operation == "plan":
        return plan(header)
    if operation not in ("deliver", "verify") or (probe_only and operation != "verify"):
        raise ValueError("This identity does not allow that operation")
    return deliver(header, stream, directory, verify_only=operation == "verify")


def deadline(*_):
    raise TimeoutError("Operation deadline exceeded")


def disconnected(*_):
    raise ConnectionAbortedError("SSH disconnected before mutation")


def main():
    signal.signal(signal.SIGALRM, deadline)
    signal.signal(signal.SIGHUP, disconnected)
    signal.alarm(120)
    os.umask(0o077)
    directory = ROOT / "delivery"
    directory.mkdir(mode=0o700, exist_ok=True)
    with (directory / ".lock").open("a") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        raw = sys.stdin.buffer.readline(8193)
        if len(raw) > 8192 or not raw.endswith(b"\n"):
            raise ValueError("Invalid operation header")
        header = json.loads(raw)
        if not isinstance(header, dict):
            raise ValueError("Operation header must be an object")
        with tempfile.TemporaryDirectory(prefix="incoming-", dir=directory) as temporary:
            return handle(header, sys.stdin.buffer, Path(temporary), sys.argv[1:] == ["--probe-only"])


if __name__ == "__main__":
    try:
        result = main()
    except StaleRelease:
        result = {"status": "stale", "message": "A newer main commit exists; no stale release was promoted"}
    except Exception as error:
        # No exception text, request/header, subprocess command, or token is logged.
        result = {"status": "failed", "error_type": type(error).__name__, "phase": PHASE}
        if PHASE == "wire_transfer":
            result["bytes_received"] = TRANSFER_BYTES
        if isinstance(error, urllib.error.HTTPError):
            result["http_status"] = error.code
    signal.alarm(0)
    print(json.dumps(result), flush=True)
    raise SystemExit(1 if result["status"] == "failed" else 0)
