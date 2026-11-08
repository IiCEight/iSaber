"""Prepare verified offline images and publish them using an existing host login."""
import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import subprocess
import sys
import tempfile

sys.dont_write_bytecode = True
from publish_images import REGISTRY, NAMESPACE, SOURCES, DIGEST

ROOT = Path(__file__).resolve().parents[1]
METADATA_FILES = (
    "infra/release_bundle.py", "infra/publish_images.py",
    "contracts/openapi.yaml", "backend/internal/migrations/migrations.go",
)
MAX_BYTES = 4 * 1024**3
MAX_FILES = 1000


def run(arguments, timeout=900):
    try:
        result = subprocess.run(arguments, capture_output=True, check=False, timeout=timeout)
    except subprocess.TimeoutExpired:
        raise RuntimeError(f"{arguments[0]} operation exceeded {timeout}s") from None
    if result.returncode:
        raise RuntimeError(f"{arguments[0]} {arguments[1]} failed (exit {result.returncode})")
    return result.stdout


def digest(path):
    value = hashlib.sha256()
    with Path(path).open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            value.update(block)
    return value.hexdigest()


def parse_json(path, limit=2 * 1024 * 1024):
    if Path(path).is_symlink() or not Path(path).is_file():
        raise ValueError("Metadata must be a regular file")
    if Path(path).stat().st_size > limit:
        raise ValueError("Metadata exceeds the size limit")
    return json.loads(Path(path).read_bytes())


def validate_header(bundle, expected_commit=None):
    if bundle.get("format_version") != 1 or bundle.get("kind") != "isaber-image-bundle":
        raise ValueError("Unsupported release bundle")
    if not re.fullmatch(r"[0-9a-f]{40}", bundle.get("source_commit", "")):
        raise ValueError("Invalid source commit")
    if expected_commit and bundle["source_commit"] != expected_commit:
        raise ValueError("Bundle does not match the trusted workflow commit")
    for key in ("ci_run_id", "bundle_run_id", "bundle_run_attempt"):
        if not re.fullmatch(r"[1-9][0-9]*", str(bundle.get(key, ""))):
            raise ValueError("Invalid workflow identity")
    if bundle.get("platform") != "linux/amd64" or set(bundle.get("images", {})) != set(SOURCES):
        raise ValueError("Unexpected image set or platform")


def verify(directory, expected_commit=None):
    directory = Path(directory)
    bundle = parse_json(directory / "bundle.json")
    validate_header(bundle, expected_commit)
    files = bundle.get("files", {})
    if not isinstance(files, dict) or len(files) > MAX_FILES or not set(METADATA_FILES).issubset(files):
        raise ValueError("Invalid bundle file inventory")
    total = 0
    actual = set()
    for path in directory.rglob("*"):
        if path.is_symlink():
            raise ValueError("Bundle must not contain symlinks")
        if path.is_file():
            actual.add(path.relative_to(directory).as_posix())
    if actual != set(files) | {"bundle.json"}:
        raise ValueError("Bundle contains missing or unexpected files")
    for relative, record in files.items():
        parts = PurePosixPath(relative)
        if parts.is_absolute() or ".." in parts.parts or "\\" in relative or parts.as_posix() != relative:
            raise ValueError("Unsafe bundle path")
        if relative not in METADATA_FILES and not relative.startswith(tuple(f"images/{name}/" for name in SOURCES)):
            raise ValueError("Unexpected bundle path")
        path = directory / relative
        size = path.stat().st_size
        total += size
        if size != record.get("size") or total > MAX_BYTES or digest(path) != record.get("sha256"):
            raise ValueError("Bundle file integrity check failed")
    if digest(directory / "contracts/openapi.yaml") != bundle.get("contract_sha256"):
        raise ValueError("Contract identity differs from bundle metadata")
    version = re.search(r"const RequiredVersion int64 = ([0-9]+)", (directory / "backend/internal/migrations/migrations.go").read_text())
    if not version or int(version[1]) != bundle.get("migration_version"):
        raise ValueError("Migration version differs from bundle metadata")
    for name, image in bundle["images"].items():
        manifest_path = directory / "images" / name / "manifest.json"
        if "sha256:" + digest(manifest_path) != image.get("manifest_digest"):
            raise ValueError("Image manifest identity mismatch")
        manifest = parse_json(manifest_path)
        config_digest = manifest.get("config", {}).get("digest", "")
        if not DIGEST.fullmatch(config_digest) or config_digest != image.get("image_id"):
            raise ValueError("Image configuration identity mismatch")
        config_path = manifest_path.parent / config_digest.split(":")[1]
        config = parse_json(config_path, 4 * 1024 * 1024)
        if digest(config_path) != config_digest.split(":")[1] or config.get("os") != "linux" or config.get("architecture") != "amd64":
            raise ValueError("Image configuration or platform mismatch")
        for layer in manifest.get("layers", []):
            layer_digest = layer.get("digest", "")
            if not DIGEST.fullmatch(layer_digest) or digest(manifest_path.parent / layer_digest.split(":")[1]) != layer_digest.split(":")[1]:
                raise ValueError("Image layer identity mismatch")
    upstream = bundle["images"]["postgres"].get("upstream_reference", "")
    if not re.fullmatch(r"(?:docker.io/library/)?postgres@sha256:[0-9a-f]{64}", upstream):
        raise ValueError("Missing official PostgreSQL provenance")
    return bundle


def export(directory, environment, transport="docker-daemon"):
    directory = Path(directory)
    if directory.exists():
        raise ValueError("Bundle output directory must not already exist")
    directory.mkdir(parents=True, mode=0o700)
    version = re.search(r"const RequiredVersion int64 = ([0-9]+)", (ROOT / METADATA_FILES[3]).read_text())
    bundle = {
        "format_version": 1, "kind": "isaber-image-bundle", "platform": "linux/amd64",
        "source_commit": environment.get("RELEASE_SHA", ""),
        "ci_run_id": environment.get("VERIFIED_CI_RUN_ID", ""),
        "bundle_run_id": environment.get("GITHUB_RUN_ID", ""),
        "bundle_run_attempt": environment.get("GITHUB_RUN_ATTEMPT", ""),
        "contract_sha256": digest(ROOT / "contracts/openapi.yaml"),
        "migration_version": int(version[1]) if version else None, "images": {},
    }
    for name, source in SOURCES.items():
        print(f"Exporting tested {name} image", flush=True)
        with tempfile.TemporaryDirectory(prefix="isaber-export-") as scratch:
            source_ref = transport + ":" + ("localhost/" if transport == "containers-storage" and name != "postgres" else "") + source
            if transport == "docker-daemon":
                # Ubuntu Skopeo's older daemon client cannot talk to Docker 29.
                # A platform-specific Docker archive avoids that SDK/API coupling.
                saved = Path(scratch) / "image.tar"
                run(["docker", "image", "save", source, "--platform", "linux/amd64", "--output", str(saved)], timeout=120)
                source_ref = "docker-archive:" + str(saved)
            source_config = run(["skopeo", "inspect", "--config", "--raw", source_ref])
            destination = directory / "images" / name
            destination.parent.mkdir(exist_ok=True)
            run(["skopeo", "--override-os", "linux", "--override-arch", "amd64", "copy", "--dest-compress", source_ref, "dir:" + str(destination)])
        manifest = parse_json(destination / "manifest.json")
        identity = "sha256:" + hashlib.sha256(source_config).hexdigest()
        if manifest.get("config", {}).get("digest") != identity:
            raise ValueError("Export changed the tested image configuration")
        bundle["images"][name] = {"image_id": identity, "manifest_digest": "sha256:" + digest(destination / "manifest.json")}
    engine = "podman" if transport == "containers-storage" else "docker"
    upstream = json.loads(run([engine, "image", "inspect", "postgres:16", "--format", "{{json .RepoDigests}}"]))
    bundle["images"]["postgres"]["upstream_reference"] = next((value for value in upstream if re.fullmatch(r"(?:docker.io/library/)?postgres@sha256:[0-9a-f]{64}", value)), "")
    for relative in METADATA_FILES:
        destination = directory / relative
        destination.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(ROOT / relative, destination)
    bundle["files"] = {path.relative_to(directory).as_posix(): {"size": path.stat().st_size, "sha256": digest(path)} for path in directory.rglob("*") if path.is_file()}
    (directory / "bundle.json").write_text(json.dumps(bundle, indent=2) + "\n")
    verify(directory, bundle["source_commit"])
    print("Verified release bundle is ready; images are not published yet", flush=True)
    return bundle


def publish(directory, output, expected_commit, authfile):
    directory = Path(directory)
    bundle = verify(directory, expected_commit)
    authfile = Path(authfile)
    if not authfile.is_file() or authfile.stat().st_mode & 0o077:
        raise ValueError("A private existing registry auth file is required")
    release = {key: bundle[key] for key in ("source_commit", "ci_run_id", "platform", "migration_version", "contract_sha256")}
    release.update(format_version=1, images={}, bundle_run_id=bundle["bundle_run_id"], bundle_run_attempt=bundle["bundle_run_attempt"])
    for name, image in bundle["images"].items():
        repository = f"{REGISTRY}/{NAMESPACE}/{name}"
        tag = repository + ":sha-" + bundle["source_commit"]
        reference = repository + "@" + image["manifest_digest"]
        print(f"Publishing {name} from verified local bundle", flush=True)
        run(["skopeo", "copy", "--preserve-digests", "--authfile", str(authfile), "dir:" + str(directory / "images" / name), "docker://" + tag])
        remote = run(["skopeo", "inspect", "--config", "--raw", "--authfile", str(authfile), "docker://" + reference])
        if "sha256:" + hashlib.sha256(remote).hexdigest() != image["image_id"]:
            raise ValueError("Published configuration differs from tested content")
        release["images"][name] = {"reference": reference, "image_id": image["image_id"]}
        if name == "postgres":
            release["images"][name]["upstream_reference"] = image["upstream_reference"]
        print(f"Verified published {name}: {reference}", flush=True)
    output = Path(output)
    output.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    with tempfile.NamedTemporaryFile(mode="w", dir=output.parent, delete=False) as destination:
        json.dump(release, destination, indent=2)
        temporary = Path(destination.name)
    try:
        os.replace(temporary, output)
    finally:
        temporary.unlink(missing_ok=True)
    print("Completed release manifest:", output, flush=True)


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    commands = parser.add_subparsers(dest="command", required=True)
    prepare = commands.add_parser("export")
    prepare.add_argument("directory")
    check = commands.add_parser("verify")
    check.add_argument("directory")
    check.add_argument("--commit", required=True)
    ship = commands.add_parser("publish")
    ship.add_argument("directory")
    ship.add_argument("--commit", required=True)
    ship.add_argument("--output", required=True)
    ship.add_argument("--authfile", default=str(Path.home() / ".docker/config.json"))
    arguments = parser.parse_args()
    try:
        if arguments.command == "export":
            export(arguments.directory, os.environ)
        elif arguments.command == "verify":
            verify(arguments.directory, arguments.commit)
        else:
            publish(arguments.directory, arguments.output, arguments.commit, arguments.authfile)
    except (ValueError, RuntimeError, OSError) as error:
        print(f"Release bundle failed: {error}", file=sys.stderr)
        raise SystemExit(1)
