"""Operator installs an independent, read-only monitoring identity on JD."""
import argparse
import os
from pathlib import Path
import pwd
import re
import shutil
import subprocess

HOME = Path("/var/lib/isaber-monitor")
COMMAND = "/usr/bin/timeout --kill-after=5s 150s /usr/bin/python3 -I /opt/isaber-staging/config/backup_health.py"


def authorized_key(value):
    parts = value.strip().split()
    if len(parts) < 2 or parts[0] != "ssh-ed25519" or not re.fullmatch(r"[A-Za-z0-9+/=]+", parts[1]):
        raise ValueError("Require an Ed25519 public key")
    return 'restrict,command="/usr/bin/python3 -I /usr/local/libexec/isaber-monitor-ssh" ' + " ".join(parts[:2]) + " isaber-backup-monitor\n"


def install(public):
    if os.geteuid() != 0: raise ValueError("Use the existing root operator connection")
    key = authorized_key(public)
    try:
        account = pwd.getpwnam("isaber-monitor")
        if account.pw_dir != str(HOME) or account.pw_uid == 0 or account.pw_shell != "/bin/sh":
            raise ValueError("Existing monitoring account differs")
    except KeyError:
        subprocess.run(["useradd", "--system", "--user-group", "--create-home", "--home-dir", str(HOME), "--shell", "/bin/sh", "isaber-monitor"], check=True)
    os.chown(HOME, 0, 0); HOME.chmod(0o755)
    ssh = HOME / ".ssh"; ssh.mkdir(mode=0o755, exist_ok=True); os.chown(ssh, 0, 0); ssh.chmod(0o755)
    keys = ssh / "authorized_keys"; keys.write_text(key); os.chown(keys, 0, 0); keys.chmod(0o644)
    entry = Path("/usr/local/libexec/isaber-monitor-ssh"); entry.parent.mkdir(mode=0o755, exist_ok=True)
    shutil.copyfile(Path(__file__).resolve().parent / "monitor_entry.py", entry)
    os.chown(entry, 0, 0); entry.chmod(0o644)
    policy = Path("/etc/sudoers.d/isaber-monitor"); pending = policy.with_suffix(".pending")
    pending.write_text("isaber-monitor ALL=(root) NOPASSWD: " + COMMAND + "\n"); pending.chmod(0o440)
    subprocess.run(["visudo", "-c", "-f", str(pending)], check=True)
    os.replace(pending, policy)
    subprocess.run(["visudo", "-c"], check=True, stdout=subprocess.DEVNULL)
    print("Read-only monitoring identity installed; deployment/upload privileges unchanged")


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--public-key", type=Path, required=True)
    args = parser.parse_args()
    install(args.public_key.read_text())
