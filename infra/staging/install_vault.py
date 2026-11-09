"""Operator-only setup; public keys and recipient only, no recovery identity."""
import argparse
import json
import os
from pathlib import Path
import pwd
import re
import shutil
import subprocess

ROOT = Path("/var/lib/isaber-backup-vault")
HOME = Path("/var/lib/isaber-vault")
COMMAND = "/usr/bin/timeout --kill-after=15s 1800s /usr/bin/python3 -I /usr/local/lib/isaber-vault/vault.py receive"


def public_key(value):
    parts = value.strip().split()
    if len(parts) < 2 or parts[0] != "ssh-ed25519" or not re.fullmatch(r"[A-Za-z0-9+/=]+", parts[1]):
        raise ValueError("Require an Ed25519 public key")
    return " ".join(parts[:2])


def install(source, upload, signing, recipient):
    if os.geteuid() != 0: raise ValueError("Use the existing operator sudo connection")
    upload, signing = public_key(upload), public_key(signing)
    if not re.fullmatch(r"age1[0-9a-z]{58}", recipient): raise ValueError("Invalid recipient")
    try:
        account = pwd.getpwnam("isaber-vault")
        if account.pw_dir != str(HOME) or account.pw_uid == 0 or account.pw_shell != "/bin/sh":
            raise ValueError("Existing vault account identity differs")
    except KeyError:
        subprocess.run(["useradd", "--system", "--user-group", "--create-home", "--home-dir", str(HOME), "--shell", "/bin/sh", "isaber-vault"], check=True)
    os.chown(HOME, 0, 0); HOME.chmod(0o755)
    (HOME / ".ssh").mkdir(mode=0o755, exist_ok=True)
    os.chown(HOME / ".ssh", 0, 0); (HOME / ".ssh").chmod(0o755)
    keyfile = HOME / ".ssh/authorized_keys"
    keyfile.write_text('restrict,command="/usr/bin/python3 -I /usr/local/libexec/isaber-vault-ssh" ' + upload + " isaber-backup-upload\n")
    os.chown(keyfile, 0, 0); keyfile.chmod(0o644)
    code = Path("/usr/local/lib/isaber-vault")
    code.mkdir(mode=0o755, parents=True, exist_ok=True)
    for name in ("vault.py", "vault_protocol.py"):
        shutil.copyfile(source / name, code / name)
        os.chown(code / name, 0, 0); (code / name).chmod(0o644)
    entry = Path("/usr/local/libexec/isaber-vault-ssh")
    entry.parent.mkdir(mode=0o755, exist_ok=True)
    shutil.copyfile(source / "vault_entry.py", entry); os.chown(entry, 0, 0); entry.chmod(0o644)
    ROOT.mkdir(mode=0o700, exist_ok=True)
    os.chown(ROOT, 0, 0); ROOT.chmod(0o700)
    (ROOT / "allowed_signers").write_text("isaber-jd-staging " + signing + "\n")
    (ROOT / "policy.json").write_text(json.dumps({"recipient": recipient}) + "\n")
    for name in ("allowed_signers", "policy.json"): (ROOT / name).chmod(0o600)
    policy = Path("/etc/sudoers.d/isaber-vault")
    temporary = policy.with_suffix(".pending")
    temporary.write_text("isaber-vault ALL=(root) NOPASSWD: " + COMMAND + "\n"); temporary.chmod(0o440)
    subprocess.run(["visudo", "-c", "-f", str(temporary)], check=True)
    os.replace(temporary, policy)
    subprocess.run(["visudo", "-c"], check=True, stdout=subprocess.DEVNULL)
    print("Encrypted append-only vault installed; no public listener added")


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--upload-public-key", type=Path, required=True)
    parser.add_argument("--signing-public-key", type=Path, required=True)
    parser.add_argument("--recipient", type=Path, required=True)
    args = parser.parse_args()
    install(Path(__file__).resolve().parent, args.upload_public_key.read_text(), args.signing_public_key.read_text(), args.recipient.read_text().strip())
