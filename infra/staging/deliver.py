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


def send(archive, header, directory):
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
    operation = "probe" if header["operation"] == "probe" or os.environ.get("STAGING_PROBE_ONLY") == "true" else "deliver"
    arguments = ["ssh", "-i", str(key), "-o", "IdentitiesOnly=yes", "-o", "BatchMode=yes", "-o", "ForwardAgent=no", "-o", "ClearAllForwardings=yes", "-o", "StrictHostKeyChecking=yes", "-o", "HostKeyAlias=isaber-jd-staging", "-o", "UserKnownHostsFile=" + str(hosts), "-o", "ConnectTimeout=20", "-o", "ServerAliveInterval=15", "-o", "ServerAliveCountMax=4", "-p", port, "isaber-deploy@" + host, operation]
    started = time.monotonic()
    with tempfile.TemporaryFile() as errors:
        process = subprocess.Popen(arguments, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=errors)
        watchdog = threading.Timer(2800, process.kill)
        watchdog.start()
        try:
            try:
                process.stdin.write(json.dumps(header).encode() + b"\n")
                with archive.open("rb") as source:
                    for block in iter(lambda: source.read(1024 * 1024), b""):
                        process.stdin.write(block)
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
                raise RuntimeError("Restricted SSH returned no valid bounded result")
            result = json.loads(raw)
            if process.returncode or result.get("status") not in ("probe", "plan", "verified", "deployed", "already_current", "stale"):
                raise RuntimeError("Server rejected the staging operation: " + str(result.get("error_type", "unknown")))
        finally:
            watchdog.cancel()
            if process.poll() is None:
                process.kill()
                process.wait()
    print(f"Restricted staging operation finished in {time.monotonic() - started:.1f}s", flush=True)
    return result


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("operation", choices=("probe", "verify", "deliver"))
    parser.add_argument("--run-id", type=int)
    parser.add_argument("--output", type=Path)
    options = parser.parse_args()
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
            empty = directory / "empty"; empty.touch()
            plan = send(empty, {"operation": "plan", "run_id": options.run_id, "token": token}, directory)
            if plan["status"] == "stale":
                result = plan
                header = None
            else:
                content = directory / "content"
                extract(archive, content)
                metadata = json.loads((content / "bundle.json").read_text())
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
                archive = thin
                value = hashlib.sha256()
                with archive.open("rb") as source:
                    for block in iter(lambda: source.read(1024 * 1024), b""):
                        value.update(block)
                header = {"operation": options.operation, "run_id": options.run_id, "token": token, "size": archive.stat().st_size, "digest": "sha256:" + value.hexdigest()}
        if header is not None:
            print(f"Sending {archive.stat().st_size / 1024**2:.1f} MiB through restricted SSH", flush=True)
            result = send(archive, header, directory)
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
        raise SystemExit(1)
