"""GitHub-runner client: download a verified ZIP, stream it over restricted SSH."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import threading
import tarfile
import time
import urllib.request
import urllib.error

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))
from relay_release import REPOSITORY, NoRedirect, check_run
from receive_artifact import StorageRedirect, validate_url, extract

DELIVERY_DEADLINE_SECONDS = 2700
MAX_ATTEMPTS = 3


class DeliveryError(RuntimeError):
    def __init__(self, message, *, retryable=False):
        super().__init__(message)
        self.retryable = retryable


def report_progress(stopped, progress, size, started):
    while not stopped.wait(15):
        elapsed = max(time.monotonic() - started, 0.001)
        sent = progress["sent"]
        stage = "uploading" if sent < size else "waiting for server verification/promotion"
        transferred = sent - progress.get("resumed", 0)
        print(f"SSH {stage}: {sent / 1024**2:.1f}/{size / 1024**2:.1f} MiB accounted for (resumed + submitted), {elapsed:.0f}s elapsed, {transferred / 1024 / elapsed:.1f} KiB/s average this connection", flush=True)


def transient_result(result):
    # Never replay a publication/promotion or an integrity/authorization rejection.
    return (result.get("phase") in ("source_metadata", "cache_seed", "wire_transfer", "bundle_verification")
            and (result.get("error_type") in ("TimeoutError", "ConnectionAbortedError", "ConnectionResetError", "URLError")
                 or result.get("http_status") in (429, 500, 502, 503, 504)))


def github(path, token):
    request = urllib.request.Request(f"https://api.github.com/repos/{REPOSITORY}/" + path, headers={"Authorization": "Bearer " + token, "Accept": "application/vnd.github+json"})
    with urllib.request.build_opener(NoRedirect).open(request, timeout=30) as response:
        return json.load(response)


def download(run_id, destination, token):
    run = github(f"actions/runs/{run_id}", token)
    check_run(run, None, None)
    attempt = run["run_attempt"]
    items = github(f"actions/runs/{run_id}/artifacts?per_page=100", token)["artifacts"]
    matches = [item for item in items if not item.get("expired") and re.fullmatch(r"bundle-[0-9a-f]{40}-" + str(attempt), item.get("name", ""))]
    if len(matches) != 1:
        raise ValueError("Expected the current successful workflow attempt's bundle")
    artifact = matches[0]
    expected = artifact.get("digest", "")
    size = artifact.get("size_in_bytes", 0)
    if not re.fullmatch(r"sha256:[0-9a-f]{64}", expected) or type(size) is not int or not 0 < size <= 512 * 1024**2:
        raise ValueError("Missing bounded artifact integrity receipt")
    request = urllib.request.Request(f"https://api.github.com/repos/{REPOSITORY}/actions/artifacts/{artifact['id']}/zip", headers={"Authorization": "Bearer " + token})
    try:
        urllib.request.build_opener(NoRedirect).open(request, timeout=30)
        raise ValueError("Artifact API did not return a redirect")
    except urllib.error.HTTPError as error:
        if error.code != 302:
            raise RuntimeError("Artifact redirect request failed") from None
        url = error.headers.get("Location", "")
        error.close()
    validate_url(url)
    value = hashlib.sha256()
    total = 0
    with urllib.request.build_opener(StorageRedirect).open(url, timeout=60) as source, destination.open("xb") as output:
        destination.chmod(0o600)
        for block in iter(lambda: source.read(1024 * 1024), b""):
            total += len(block)
            if total > size:
                raise ValueError("Artifact exceeds its receipt size")
            output.write(block)
            value.update(block)
    if total != size or "sha256:" + value.hexdigest() != expected:
        raise ValueError("Downloaded artifact differs from GitHub's receipt")
    print(f"Downloaded and verified {total / 1024**2:.1f} MiB on the GitHub runner", flush=True)


def send(archive, header, directory, *, timeout=DELIVERY_DEADLINE_SECONDS):
    if timeout <= 0:
        raise DeliveryError("Delivery exceeded its overall deadline")
    key, hosts = directory / "identity", directory / "known_hosts"
    key.write_text(os.environ["STAGING_SSH_KEY"].strip() + "\n")
    key.chmod(0o600)
    public = os.environ["STAGING_SSH_HOST_KEY"].strip()
    if not re.fullmatch(r"ssh-ed25519 [A-Za-z0-9+/=]+(?: [^\r\n]*)?", public):
        raise ValueError("A pinned Ed25519 host key is required")
    hosts.write_text("isaber-jd-staging " + " ".join(public.split()[:2]) + "\n")
    host = os.environ["STAGING_HOST"]
    port = os.environ.get("STAGING_PORT", "22")
    if not re.fullmatch(r"[A-Za-z0-9.-]+", host) or not re.fullmatch(r"[0-9]{1,5}", port) or not 1 <= int(port) <= 65535:
        raise ValueError("Invalid SSH destination")
    operation = "probe" if os.environ.get("STAGING_PROBE_ONLY") == "true" else "deliver"
    arguments = ["ssh", "-i", str(key), "-o", "IdentitiesOnly=yes", "-o", "BatchMode=yes", "-o", "ForwardAgent=no", "-o", "ClearAllForwardings=yes", "-o", "StrictHostKeyChecking=yes", "-o", "HostKeyAlias=isaber-jd-staging", "-o", "UserKnownHostsFile=" + str(hosts), "-o", "ConnectTimeout=20", "-o", "ServerAliveInterval=15", "-o", "ServerAliveCountMax=4", "-p", port, "isaber-deploy@" + host, operation]
    started = time.monotonic()
    size = archive.stat().st_size
    offset = header.get("offset", 0)
    if type(offset) is not int or not 0 <= offset <= size:
        raise ValueError("Invalid local resume offset")
    progress = {"sent": offset, "resumed": offset}
    stopped = threading.Event()
    with tempfile.TemporaryFile() as errors:
        process = subprocess.Popen(arguments, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=errors)
        watchdog = threading.Timer(timeout, process.kill)
        watchdog.start()
        reporter = threading.Thread(target=report_progress, args=(stopped, progress, size, started), daemon=True)
        reporter.start()
        try:
            try:
                process.stdin.write(json.dumps(header).encode() + b"\n")
                with archive.open("rb") as source:
                    source.seek(offset)
                    for block in iter(lambda: source.read(64 * 1024), b""):
                        process.stdin.write(block)
                        progress["sent"] += len(block)
                process.stdin.flush()
                print("SSH upload finished; waiting for server verification and the staging operation", flush=True)
            except BrokenPipeError:
                # A stale request may be rejected before its body is sent.
                pass
            finally:
                try:
                    process.stdin.close()
                except BrokenPipeError:
                    pass
            limit = 2 * 1024**2 if header["operation"] == "plan" else 128 * 1024
            raw = process.stdout.read(limit + 1)
            process.wait()
            if len(raw) > limit or not raw:
                errors.seek(0)
                diagnostic = errors.read(64 * 1024).lower()
                reason = "connection interrupted or timed out"
                denied = any(marker in diagnostic for marker in (b"permission denied", b"host key verification failed", b"remote host identification has changed"))
                if denied:
                    reason = "SSH authentication or pinned host-key verification failed"
                # An incomplete body cannot have reached publication. A missing
                # result after the full body is ambiguous and needs operator checks.
                safe_retry = not denied and len(raw) == 0 and (header["operation"] == "plan" or progress["sent"] < size)
                raise DeliveryError(f"Restricted SSH returned no valid bounded result ({reason}; exit {process.returncode}, {progress['sent']}/{size} bytes submitted)", retryable=safe_retry)
            try:
                result = json.loads(raw)
            except (ValueError, UnicodeError):
                raise DeliveryError("Restricted SSH returned invalid JSON; check staging state before retrying") from None
            if not isinstance(result, dict):
                raise DeliveryError("Restricted SSH returned an invalid result")
            if process.returncode or result.get("status") not in ("probe", "plan", "verified", "deployed", "already_current", "stale"):
                print(json.dumps({key: result[key] for key in ("status", "error_type", "phase", "http_status", "bytes_received") if key in result}), flush=True)
                raise DeliveryError("Server rejected the staging operation: " + str(result.get("error_type", "unknown")), retryable=transient_result(result))
        finally:
            stopped.set()
            reporter.join(timeout=1)
            watchdog.cancel()
            if process.poll() is None:
                process.kill()
                process.wait()
            process.stdout.close()
    print(f"Restricted staging operation finished in {time.monotonic() - started:.1f}s", flush=True)
    return result


def deliver_bundle(archive, directory, operation, run_id, token, *, deadline):
    content = directory / "content"
    extract(archive, content)
    metadata = json.loads((content / "bundle.json").read_text())
    empty = directory / "empty"
    empty.touch()
    for attempt in range(1, MAX_ATTEMPTS + 1):
        print(f"Staging delivery attempt {attempt}/{MAX_ATTEMPTS}: requesting a fresh authenticated cache plan", flush=True)
        try:
            plan = send(empty, {"operation": "plan", "run_id": run_id, "token": token}, directory, timeout=deadline - time.monotonic())
            if plan["status"] == "stale":
                return plan
            if metadata != plan["bundle"]:
                raise ValueError("Server-authenticated metadata differs from the verified runner bundle")
            thin = directory / "missing.tar"
            with tarfile.open(thin, "w", format=tarfile.USTAR_FORMAT) as output:
                for relative in plan["missing"]:
                    if relative not in metadata["files"]:
                        raise ValueError("Unknown missing-file request")
                    entry = output.gettarinfo(str(content / relative), arcname=relative)
                    entry.mtime = entry.uid = entry.gid = 0
                    entry.uname = entry.gname = ""
                    with (content / relative).open("rb") as source:
                        output.addfile(entry, source)
            print(f"JD cache leaves {thin.stat().st_size / 1024**2:.1f} MiB to transfer from the original {archive.stat().st_size / 1024**2:.1f} MiB bundle", flush=True)
            value = hashlib.sha256()
            with thin.open("rb") as source:
                for block in iter(lambda: source.read(1024 * 1024), b""):
                    value.update(block)
            header = {"operation": operation, "run_id": run_id, "token": token, "size": thin.stat().st_size, "digest": "sha256:" + value.hexdigest()}
            partial = plan.get("partial_transfer")
            if (isinstance(partial, dict) and partial.get("digest") == header["digest"]
                    and partial.get("size") == header["size"]
                    and type(partial.get("offset")) is int and 0 < partial["offset"] <= header["size"]):
                prefix = hashlib.sha256()
                remaining = partial["offset"]
                with thin.open("rb") as source:
                    while remaining:
                        block = source.read(min(remaining, 1024 * 1024))
                        prefix.update(block); remaining -= len(block)
                if prefix.hexdigest() == partial.get("prefix_sha256"):
                    header.update(offset=partial["offset"], prefix_sha256=prefix.hexdigest())
                    print(f"Resuming after {partial['offset'] / 1024**2:.1f} MiB already received; prefix matches the local archive", flush=True)
            return send(thin, header, directory, timeout=deadline - time.monotonic())
        except DeliveryError as error:
            delay = 5 * attempt
            if not error.retryable or attempt == MAX_ATTEMPTS or time.monotonic() + delay >= deadline:
                raise
            print(f"Transient delivery failure: {error}. Retrying in {delay}s with a fresh plan; completed verified cache entries will be reused.", flush=True)
            time.sleep(delay)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("operation", choices=("probe", "verify", "deliver"))
    parser.add_argument("--run-id", type=int)
    parser.add_argument("--output", type=Path)
    options = parser.parse_args()
    deadline = time.monotonic() + DELIVERY_DEADLINE_SECONDS
    with tempfile.TemporaryDirectory(prefix="isaber-staging-delivery-") as temporary:
        directory = Path(temporary)
        archive = directory / "bundle.zip"
        if options.operation == "probe":
            content = os.urandom(8 * 1024**2)
            archive.write_bytes(content)
            header = {"operation": "probe", "size": len(content), "digest": "sha256:" + hashlib.sha256(content).hexdigest()}
        else:
            if not options.run_id or options.run_id < 1:
                raise ValueError("A source preparation --run-id is required")
            token = os.environ["GH_TOKEN"]
            download(options.run_id, archive, token)
            result = deliver_bundle(archive, directory, options.operation, options.run_id, token, deadline=deadline)
            header = None
        if header is not None:
            print(f"Sending {archive.stat().st_size / 1024**2:.1f} MiB through restricted SSH", flush=True)
            result = send(archive, header, directory, timeout=deadline - time.monotonic())
        if options.output:
            options.output.parent.mkdir(parents=True, exist_ok=True)
            options.output.write_text(json.dumps(result, indent=2) + "\n")
        if "GITHUB_OUTPUT" in os.environ:
            with open(os.environ["GITHUB_OUTPUT"], "a") as output:
                output.write("deployed=" + str(result["status"] in ("deployed", "already_current")).lower() + "\n")
                output.write("outcome=" + result["status"] + "\n")
        print(json.dumps({key: value for key, value in result.items() if key != "release"}), flush=True)


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        # Network exceptions may include signed URLs. Headers/tokens stay private.
        print(f"Staging delivery failed ({type(error).__name__}); inspect the bounded server result and protected delivery log", file=sys.stderr)
        if isinstance(error, DeliveryError):
            print(str(error), file=sys.stderr)
        raise SystemExit(1)
