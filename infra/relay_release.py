"""Operator-driven recovery: authenticated GitHub download, existing SSH, domestic push."""
import argparse
import json
import os
from pathlib import Path
import re
import shlex
import subprocess
import sys
import tarfile
import tempfile
import time

from release_bundle import verify, digest

REPOSITORY = "IiCEight/iSaber"
SSH_OPTIONS = ["-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=yes", "-o", "ForwardAgent=no", "-o", "ClearAllForwardings=yes"]


def command(arguments, timeout=900, capture=False, text_input=None):
    result = subprocess.run(arguments, input=text_input, text=True, stdout=subprocess.PIPE if capture else None, check=True, timeout=timeout)
    return result.stdout.strip() if capture else None


def github(path):
    return json.loads(command(["gh", "api", "repos/" + REPOSITORY + "/" + path], capture=True, timeout=60))


def check_run(run, jobs, ci, source_commit=None):
    if run.get("path") != ".github/workflows/publish.yml" or run.get("event") != "workflow_run" or run.get("head_branch") != "main" or run.get("conclusion") != "success" or run.get("head_repository", {}).get("full_name") != REPOSITORY:
        raise ValueError("Require a successful trusted main release-bundle workflow")
    if jobs is None:
        return
    if not any(job.get("name") == "Prepare tested image bundle" and job.get("conclusion") == "success" for job in jobs):
        raise ValueError("The tested image bundle job did not succeed")
    if ci.get("path") != ".github/workflows/ci.yml" or ci.get("event") != "push" or ci.get("head_branch") != "main" or ci.get("conclusion") != "success" or ci.get("head_sha") != (source_commit or run.get("head_sha")) or ci.get("head_repository", {}).get("full_name") != REPOSITORY:
        raise ValueError("Bundle must match successful push-to-main CI")


def ssh(program, *arguments, capture=False, text_input=None, timeout=2400):
    remote = shlex.join([program, *arguments])
    return command(["ssh", *SSH_OPTIONS, "jdcloud", remote], capture=capture, text_input=text_input, timeout=timeout)


BOOTSTRAP = r'''
import hashlib, os, pathlib, shutil, subprocess, sys, tarfile
directory=pathlib.Path(sys.argv[1]); expected=sys.argv[2]; commit=sys.argv[3]
archive=directory/'bundle.tar'
hash=hashlib.sha256()
with archive.open('rb') as source:
    for block in iter(lambda:source.read(1024*1024),b''): hash.update(block)
if hash.hexdigest()!=expected: raise SystemExit('Transfer checksum mismatch')
if not shutil.which('skopeo'): raise SystemExit('Install the reviewed skopeo package on JD first')
root=directory/'content'; root.mkdir(mode=0o700)
with tarfile.open(archive) as source:
    members=source.getmembers()
    if len(members)>1001 or sum(member.size for member in members)>4*1024**3: raise SystemExit('Archive exceeds relay limits')
    if shutil.disk_usage(directory).free < sum(member.size for member in members)+1024**3: raise SystemExit('Insufficient free staging disk')
    names=set()
    for member in members:
        path=pathlib.PurePosixPath(member.name)
        if not member.isfile() or path.is_absolute() or '..' in path.parts or '\\' in member.name or path.as_posix()!=member.name or member.name in names: raise SystemExit('Unsafe archive entry')
        names.add(member.name)
        target=root/member.name; target.parent.mkdir(parents=True,exist_ok=True,mode=0o700)
        with source.extractfile(member) as data, target.open('xb') as output: shutil.copyfileobj(data,output)
        target.chmod(0o600)
subprocess.run([sys.executable,str(root/'infra/release_bundle.py'),'verify',str(root),'--commit',commit],check=True,timeout=300)
subprocess.run(['timeout','--kill-after=10s','1800s',sys.executable,str(root/'infra/release_bundle.py'),'publish',str(root),'--commit',commit,'--output',str(directory/'release.json')],check=True,timeout=1820)
'''


def relay(run_id):
    run = github(f"actions/runs/{run_id}")
    check_run(run, None, None)
    attempt = run.get("run_attempt")
    if not isinstance(attempt, int) or attempt < 1:
        raise ValueError("Invalid workflow attempt")
    artifacts = github(f"actions/runs/{run_id}/artifacts?per_page=100")["artifacts"]
    candidates = [item for item in artifacts if not item.get("expired") and re.fullmatch(r"bundle-[0-9a-f]{40}-" + str(attempt), item.get("name", ""))]
    if len(candidates) != 1:
        raise ValueError("Expected one unexpired bundle for the current workflow attempt")
    artifact_name = candidates[0]["name"]
    commit = artifact_name.split("-")[1]
    with tempfile.TemporaryDirectory(prefix="isaber-relay-") as directory:
        root = Path(directory)
        content = root / "content"
        print(f"Downloading {artifact_name} through the authenticated local GitHub CLI", flush=True)
        command(["gh", "run", "download", str(run_id), "--repo", REPOSITORY, "--name", artifact_name, "--dir", str(content)])
        bundle = verify(content, commit)
        if str(bundle["bundle_run_id"]) != str(run_id) or str(bundle["bundle_run_attempt"]) != str(attempt):
            raise ValueError("Bundle belongs to another workflow attempt")
        jobs = github(f"actions/runs/{run_id}/attempts/{attempt}/jobs?per_page=100")["jobs"]
        ci = github("actions/runs/" + str(bundle["ci_run_id"]))
        check_run(run, jobs, ci, commit)
        archive = root / "bundle.tar"
        with tarfile.open(archive, "w") as output:
            for relative in ["bundle.json", *sorted(bundle["files"])]:
                output.add(content / relative, arcname=relative, recursive=False)
        remote = ssh("mktemp", "-d", "/tmp/isaber-relay.XXXXXXXX", capture=True)
        if not re.fullmatch(r"/tmp/isaber-relay\.[A-Za-z0-9]+", remote):
            raise ValueError("Invalid remote temporary directory")
        try:
            print(f"Transferring {archive.stat().st_size / 1024**2:.1f} MiB to JD through existing SSH", flush=True)
            started = time.monotonic()
            command(["scp", *SSH_OPTIONS, str(archive), "jdcloud:" + remote + "/bundle.tar"])
            print(f"SSH transfer finished in {time.monotonic() - started:.1f}s; validating and publishing on JD", flush=True)
            ssh("python3", "-c", BOOTSTRAP, remote, digest(archive), commit)
            target = Path(__file__).resolve().parent / "releases" / commit / f"{run_id}-{attempt}"
            target.mkdir(parents=True, exist_ok=True, mode=0o700)
            pending = target / ".pending.json"
            command(["scp", *SSH_OPTIONS, "jdcloud:" + remote + "/release.json", str(pending)])
            release = json.loads(pending.read_text())
            if release.get("format_version") != 1 or release.get("source_commit") != commit or any(release.get(key) != bundle.get(key) for key in ("ci_run_id", "platform", "migration_version", "contract_sha256", "bundle_run_id", "bundle_run_attempt")):
                raise ValueError("Returned release metadata mismatch")
            if set(release.get("images", {})) != set(bundle["images"]):
                raise ValueError("Returned release image set mismatch")
            for name, image in bundle["images"].items():
                if release["images"][name]["image_id"] != image["image_id"] or release["images"][name]["reference"] != f"ccr.ccs.tencentyun.com/isaber/{name}@{image['manifest_digest']}":
                    raise ValueError("Returned release digest mismatch")
            pending.chmod(0o600)
            os.replace(pending, target / "release.json")
            print("Verified release manifest retained at", target / "release.json", flush=True)
            print("Images are published; no application services were deployed", flush=True)
        finally:
            ssh("python3", "-c", "import shutil,sys; shutil.rmtree(sys.argv[1])", remote, timeout=60)


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("run_id", type=int)
    arguments = parser.parse_args()
    try:
        if arguments.run_id <= 0:
            raise ValueError("A positive workflow run ID is required")
        relay(arguments.run_id)
    except (ValueError, RuntimeError, OSError, subprocess.SubprocessError) as error:
        print(f"Relay failed: {error}", file=sys.stderr)
        raise SystemExit(1)
