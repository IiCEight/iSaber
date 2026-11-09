"""Operator-only installation of the fixed SSH gateway; never run from an upload."""
import argparse
import os
from pathlib import Path
import pwd
import re
import subprocess

HOME = Path("/var/lib/isaber-deploy")
COMMAND = "/usr/bin/timeout --kill-after=15s 2700s /usr/bin/python3 /opt/isaber-staging/control/gateway.py"


def authorized_key(public, probe=False):
    parts = public.strip().split()
    if len(parts) < 2 or parts[0] != "ssh-ed25519" or not re.fullmatch(r"[A-Za-z0-9+/=]+", parts[1]):
        raise ValueError("Require an Ed25519 public key")
    command = "/usr/bin/python3 /usr/local/libexec/isaber-staging-ssh" + (" probe" if probe else "")
    return 'restrict,command="' + command + '" ' + " ".join(parts[:2]) + (" isaber-probe" if probe else " isaber-delivery") + "\n"


def install(delivery, probe=None):
    if os.geteuid() != 0:
        raise ValueError("This installation requires the existing operator root connection")
    try:
        account = pwd.getpwnam("isaber-deploy")
        if account.pw_dir != str(HOME) or account.pw_uid == 0 or account.pw_shell != "/bin/sh":
            raise ValueError("Existing deployment account does not match the reviewed identity")
    except KeyError:
        subprocess.run(["useradd", "--system", "--user-group", "--create-home", "--home-dir", str(HOME), "--shell", "/bin/sh", "isaber-deploy"], check=True)
    os.chown(HOME, 0, 0)
    HOME.chmod(0o755)
    ssh = HOME / ".ssh"
    ssh.mkdir(mode=0o755, exist_ok=True)
    os.chown(ssh, 0, 0)
    ssh.chmod(0o755)
    keys = authorized_key(delivery)
    if probe:
        keys += authorized_key(probe, True)
    path = ssh / "authorized_keys"
    path.write_text(keys)
    os.chown(path, 0, 0)
    path.chmod(0o644)
    policy = Path("/etc/sudoers.d/isaber-staging")
    pending = policy.with_suffix(".pending")
    pending.write_text("isaber-deploy ALL=(root) NOPASSWD: " + COMMAND + ", " + COMMAND + " --probe-only\n")
    pending.chmod(0o440)
    subprocess.run(["visudo", "-c", "-f", str(pending)], check=True)
    os.replace(pending, policy)
    subprocess.run(["visudo", "-c"], check=True, stdout=subprocess.DEVNULL)
    print("Restricted account installed; no shell, forwarding, or Docker-group access was granted")


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--delivery-public-key", type=Path, required=True)
    parser.add_argument("--probe-public-key", type=Path)
    args = parser.parse_args()
    install(args.delivery_public_key.read_text(), args.probe_public_key.read_text() if args.probe_public_key else None)
