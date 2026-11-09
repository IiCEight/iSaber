"""Read-only monitoring key cannot invoke delivery, a shell, or backup writes."""
import os
import sys

if sys.argv[1:] or os.environ.get("SSH_ORIGINAL_COMMAND") != "backup-health":
    print("This key permits only the fixed read-only backup-health check", file=sys.stderr)
    raise SystemExit(1)
arguments = ["/usr/bin/sudo", "-n", "/usr/bin/timeout", "--kill-after=5s", "150s", "/usr/bin/python3", "-I", "/opt/isaber-staging/config/backup_health.py"]
os.execv(arguments[0], arguments)
