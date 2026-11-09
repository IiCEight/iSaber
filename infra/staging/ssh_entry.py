"""Root-owned forced command; deployment keys never obtain a login shell."""
import os
import sys

mode = "probe" if sys.argv[1:] == ["probe"] else "deliver"
if sys.argv[1:] not in ([], ["probe"]) or os.environ.get("SSH_ORIGINAL_COMMAND") != mode:
    print("This key permits only its fixed staging operation", file=sys.stderr)
    raise SystemExit(1)
arguments = ["/usr/bin/sudo", "-n", "/usr/bin/timeout", "--kill-after=15s", "2700s", "/usr/bin/python3", "/opt/isaber-staging/control/gateway.py"]
if mode == "probe":
    arguments.append("--probe-only")
os.execv(arguments[0], arguments)
