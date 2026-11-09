"""Owner workstation: verify a vault export, decrypt locally, check on isolated JD."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tarfile
import tempfile

sys.path.insert(0, str(Path(__file__).resolve().parent))
from vault_protocol import canonical, validate_id, validate_manifest, validate_signature


def unpack(source, destination):
    """Accept exactly three bounded regular USTAR members in export order."""
    for name, maximum in (("manifest.json", 4096), ("signature", 4096), ("snapshot.age", 2 * 1024**3)):
        info = tarfile.TarInfo.frombuf(source.read(512), "utf-8", "strict")
        if info.name != name or not info.isfile() or not 0 < info.size <= maximum:
            raise ValueError("Invalid encrypted vault export")
        with (destination / name).open("xb") as output:
            size = info.size
            while size:
                block = source.read(min(size, 1024 * 1024))
                if not block: raise ValueError("Truncated encrypted export")
                output.write(block); size -= len(block)
        if len(source.read((-info.size) % 512)) != (-info.size) % 512:
            raise ValueError("Truncated archive padding")
    # USTAR end padding is bounded to one archive record.
    trailing = source.read(10241)
    if len(trailing) > 10240 or any(trailing): raise ValueError("Unexpected export members")


def verify(directory, identity, keygen):
    manifest = validate_manifest(json.loads((directory / "manifest.json").read_text()))
    validate_signature((directory / "signature").read_text())
    signers = [line.removeprefix("# isaber-backup-signer: ") for line in identity.read_text().splitlines() if line.startswith("# isaber-backup-signer: ")]
    if len(signers) != 1: raise ValueError("Recovery identity lacks its trusted signer comment")
    allowed = directory / "allowed_signers"
    allowed.write_text("isaber-jd-staging " + signers[0] + "\n")
    result = subprocess.run(["ssh-keygen", "-Y", "verify", "-f", str(allowed), "-I", "isaber-jd-staging", "-n", "isaber-backup", "-s", str(directory / "signature")], input=canonical(manifest), capture_output=True, timeout=15)
    if result.returncode: raise ValueError("Vault signature does not match the owner's pinned signer")
    recipient = subprocess.run([str(keygen), "-y", str(identity)], capture_output=True, check=True, timeout=15).stdout.decode().strip()
    if manifest["recipient"] != recipient: raise ValueError("Wrong recovery identity")
    cipher = directory / "snapshot.age"
    digest = hashlib.sha256()
    with cipher.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""): digest.update(block)
    if cipher.stat().st_size != manifest["cipher_bytes"] or digest.hexdigest() != manifest["cipher_sha256"]:
        raise ValueError("Vault ciphertext checksum mismatch")
    return manifest


def recover(identifier, identity, age, keygen, vault="tencent", target="jdcloud"):
    validate_id(identifier)
    os.umask(0o077)
    with tempfile.TemporaryDirectory(prefix="isaber-recovery-") as temporary:
        directory = Path(temporary)
        exported = directory / "export.tar"
        with exported.open("xb") as output:
            subprocess.run(["ssh", "-o", "BatchMode=yes", vault, "sudo -n python3 -I /usr/local/lib/isaber-vault/vault.py export --id " + identifier], stdout=output, stderr=subprocess.PIPE, check=True, timeout=1600)
        with exported.open("rb") as source: unpack(source, directory)
        manifest = verify(directory, identity, keygen)
        if manifest["id"] != identifier: raise ValueError("Vault returned another snapshot")
        with tempfile.TemporaryFile() as errors:
            decrypt = subprocess.Popen([str(age), "-d", "-i", str(identity), str(directory / "snapshot.age")], stdout=subprocess.PIPE, stderr=errors)
            receiver = None
            try:
                receiver = subprocess.Popen(["ssh", "-o", "BatchMode=yes", target, "python3 -I /opt/isaber-staging/config/offsite_restore.py"], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=errors)
                receiver.stdin.write(canonical(manifest))
                shutil.copyfileobj(decrypt.stdout, receiver.stdin, 1024 * 1024)
                receiver.stdin.close(); receiver.stdin = None
                if decrypt.wait(timeout=30): raise ValueError("Age authentication failed")
                response = receiver.communicate(timeout=900)[0]
                if receiver.returncode: raise RuntimeError("Isolated recovery check failed")
                # The receiver prints only a proof, never recovered database/media.
                print(response.decode().strip())
            finally:
                for process in (decrypt, receiver):
                    if process is not None and process.poll() is None: process.kill(); process.wait()


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--id", required=True)
    parser.add_argument("--identity", type=Path, required=True)
    parser.add_argument("--age", type=Path, default=Path("/usr/bin/age"))
    parser.add_argument("--keygen", type=Path, default=Path("/usr/bin/age-keygen"))
    parser.add_argument("--vault", default="tencent")
    parser.add_argument("--target", default="jdcloud")
    options = parser.parse_args()
    try:
        recover(options.id, options.identity, options.age, options.keygen, options.vault, options.target)
    except Exception as error:
        print("Offsite recovery failed (" + type(error).__name__ + "); private output omitted", file=sys.stderr)
        raise SystemExit(1)
