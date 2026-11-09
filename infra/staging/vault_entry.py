"""Forced SSH entry point: uploader has only the fixed receive operation."""
import os
import sys

if sys.argv[1:] or os.environ.get("SSH_ORIGINAL_COMMAND") != "store":
    print("This key permits only encrypted backup upload/verification", file=sys.stderr)
    raise SystemExit(1)
command = ["/usr/bin/sudo", "-n", "/usr/bin/timeout", "--kill-after=15s", "1800s", "/usr/bin/python3", "-I", "/usr/local/lib/isaber-vault/vault.py", "receive"]
os.execv(command[0], command)
