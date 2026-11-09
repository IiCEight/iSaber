"""Fixed privileged receiver: verified main artifacts, never uploaded program execution."""
import contextlib
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import signal
import sys
import tempfile
import time
import urllib.request

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))
sys.path.insert(0, str(Path(__file__).resolve().parent.parent / "config"))
from relay_release import check_run, REPOSITORY, NoRedirect
from receive_artifact import extract
from release_bundle import verify, publish
from staging import compatible_release, promote, write_private

ROOT = Path("/opt/isaber-staging")
MAX_BYTES = 512 * 1024**2


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


def receive(stream, path, size, expected):
    started = time.monotonic()
    value = hashlib.sha256()
    remaining = size
    with path.open("xb") as output:
        path.chmod(0o600)
        while remaining:
            if time.monotonic() - started > 600:
                raise TimeoutError("Transfer deadline exceeded")
            signal.alarm(45)
            block = stream.read(min(1024 * 1024, remaining))
            if not block:
                raise ValueError("Incomplete transfer")
            output.write(block)
            value.update(block)
            remaining -= len(block)
    signal.alarm(120)
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


def deliver(header, stream, directory):
    run_id = header.get("run_id")
    if type(run_id) is not int or run_id < 1:
        raise ValueError("A source preparation run is required")
    api = GitHub(header.get("token"))
    run = api.get(f"actions/runs/{run_id}")
    artifact = select_artifact(run, api.get(f"actions/runs/{run_id}/artifacts?per_page=100")["artifacts"])
    commit = artifact["name"].split("-")[1]
    require_current(api, commit)
    archive = directory / "bundle.zip"
    seconds = receive(stream, archive, artifact["size_in_bytes"], artifact["digest"])
    content = directory / "content"
    extract(archive, content)
    # Imports above are root-owned installed tools. Uploaded .py files are data
    # covered by the artifact receipt; none are imported or executed by root.
    bundle = verify(content, commit)
    attempt = run["run_attempt"]
    if bundle["format_version"] != 2 or str(bundle["bundle_run_id"]) != str(run_id) or str(bundle["bundle_run_attempt"]) != str(attempt):
        raise ValueError("Bundle is not the expected optimized workflow attempt")
    jobs = api.get(f"actions/runs/{run_id}/attempts/{attempt}/jobs?per_page=100")["jobs"]
    check_run(run, jobs, api.get("actions/runs/" + str(bundle["ci_run_id"])), commit)
    candidate = expected_release(bundle)
    current = json.loads((ROOT / "current-release.json").read_text())
    compatible_release(current, candidate)
    if current == candidate:
        return {"status": "already_current", "release": current, "transfer_seconds": seconds}
    require_current(api, commit)
    signal.alarm(2100)
    signal.signal(signal.SIGHUP, signal.SIG_IGN)
    logfile = ROOT / "delivery" / f"{run_id}-{attempt}.log"
    with logfile.open("a") as log, contextlib.redirect_stdout(log), contextlib.redirect_stderr(log):
        logfile.chmod(0o600)
        output = directory / "release.json"
        publish(content, output, commit, "/root/.docker/config.json")
        if json.loads(output.read_text()) != candidate:
            raise ValueError("Publisher metadata differs from the tested bundle")
        require_current(api, commit)
        promote(ROOT, output)
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
    if operation != "deliver" or probe_only:
        raise ValueError("This identity does not allow that operation")
    return deliver(header, stream, directory)


def main():
    signal.signal(signal.SIGALRM, lambda *_: (_ for _ in ()).throw(TimeoutError("Operation deadline exceeded")))
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
        result = {"status": "failed", "error_type": type(error).__name__}
    signal.alarm(0)
    print(json.dumps(result), flush=True)
    raise SystemExit(1 if result["status"] == "failed" else 0)
