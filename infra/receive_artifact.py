"""Download one short-lived artifact on JD. GitHub credentials stay on the operator host."""
import hashlib
import json
from pathlib import Path, PurePosixPath
import shutil
import stat
import sys
import time
import urllib.parse
import urllib.request
import zipfile

MAX_BYTES = 4 * 1024**3


def validate_url(url):
    parsed = urllib.parse.urlsplit(url)
    if parsed.scheme != "https" or parsed.username or parsed.password or parsed.port not in (None, 443) or not any((parsed.hostname or "").endswith(suffix) for suffix in (".blob.core.windows.net", ".githubusercontent.com")):
        raise ValueError("Artifact URL is not a GitHub HTTPS storage endpoint")


class StorageRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, response, code, message, headers, new_url):
        validate_url(new_url)
        return super().redirect_request(request, response, code, message, headers, new_url)


def extract(archive, root):
    with zipfile.ZipFile(archive) as source:
        members = source.infolist()
        total = sum(member.file_size for member in members)
        if len(members) > 1001 or total > MAX_BYTES:
            raise ValueError("Artifact exceeds relay limits")
        if shutil.disk_usage(root.parent).free < total + 1024**3:
            raise ValueError("Insufficient free staging disk")
        names = set()
        for member in members:
            path = PurePosixPath(member.filename)
            mode = stat.S_IFMT(member.external_attr >> 16)
            if member.is_dir() or mode not in (0, stat.S_IFREG) or path.is_absolute() or ".." in path.parts or "\\" in member.filename or path.as_posix() != member.filename or member.filename in names:
                raise ValueError("Unsafe artifact entry")
            names.add(member.filename)
        root.mkdir(mode=0o700)
        for member in members:
            target = root / member.filename
            target.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
            with source.open(member) as data, target.open("xb") as output:
                shutil.copyfileobj(data, output)
            target.chmod(0o600)


def receive(directory, payload):
    url = payload["url"]
    validate_url(url)
    expected = payload["digest"]
    import re
    if not re.fullmatch(r"sha256:[0-9a-f]{64}", expected):
        raise ValueError("Artifact must have a trusted SHA-256 receipt")
    size = payload["size_in_bytes"]
    if not isinstance(size, int) or not 0 < size <= MAX_BYTES:
        raise ValueError("Invalid artifact size")
    started = time.monotonic()
    total = 0
    checksum = hashlib.sha256()
    archive = Path(directory) / "bundle.zip"
    # A signed URL goes only through SSH stdin, never argv, logs, or a config file.
    with urllib.request.build_opener(StorageRedirect).open(url, timeout=45) as response, archive.open("xb") as output:
        archive.chmod(0o600)
        while True:
            if time.monotonic() - started > 600:
                raise TimeoutError("Artifact download exceeded ten minutes")
            block = response.read(1024 * 1024)
            if not block:
                break
            total += len(block)
            if total > size:
                raise ValueError("Artifact exceeds its trusted size")
            checksum.update(block)
            output.write(block)
    if total != size or "sha256:" + checksum.hexdigest() != expected:
        raise ValueError("Artifact checksum or size differs from GitHub receipt")
    extract(archive, Path(directory) / "content")
    if "bundle" in payload:
        metadata = Path(directory) / "content/bundle.json"
        if metadata.stat().st_size > 2 * 1024**2 or json.loads(metadata.read_text()) != payload["bundle"]:
            raise ValueError("Downloaded bundle differs from trusted metadata")
    print(f"JD downloaded and verified {total / 1024**2:.1f} MiB in {time.monotonic() - started:.1f}s", flush=True)


if __name__ == "__main__":
    try:
        receive(sys.argv[1], json.load(sys.stdin))
    except Exception as error:
        # Network exceptions can embed the signed URL. Never print their text.
        print(f"Artifact receipt failed ({type(error).__name__}); retry with a fresh signed URL or use --transport workspace", file=sys.stderr)
        raise SystemExit(1)
