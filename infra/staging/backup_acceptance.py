"""Verify nonempty DB/media recovery using only uniquely named temporary probes."""
import hashlib
import json
from pathlib import Path
import secrets

from backup import lock, snapshot, restore_check, volume_command
from staging import PROJECT, run, sql


def accept(root):
    with lock(root):
        release = json.loads((root / "current-release.json").read_text())
        image = release["images"]["frontend"]["reference"]
        token = secrets.token_hex(8)
        table = "staging_backup_probe_" + token
        filename = ".backup-probe-" + token
        payload = "Synthetic staging backup probe " + token
        created_table = created_file = False
        try:
            sql(root, f'CREATE TABLE "{table}" (payload text NOT NULL); INSERT INTO "{table}" VALUES (\'{payload}\');')
            created_table = True
            script = "require('node:fs').writeFileSync(" + json.dumps("/source/" + filename) + "," + json.dumps(payload) + ",{flag:'wx',mode:0o600})"
            run(volume_command(image, PROJECT + "_uploads", "/source", "node", "-e", script, writable=True))
            created_file = True
            directory = snapshot(root)
            metadata = json.loads((directory / "backup.json").read_text())
            if metadata["row_counts"].get(table) != 1 or metadata["uploads"].get(filename, {}).get("sha256") != hashlib.sha256(payload.encode()).hexdigest():
                raise ValueError("Nonempty recovery probes were not captured")
            restore_check(root, directory)
            print("Nonempty synthetic database/media recovery passed", flush=True)
        finally:
            if created_table:
                sql(root, f'DROP TABLE "{table}";')
            if created_file:
                script = "require('node:fs').unlinkSync(" + json.dumps("/source/" + filename) + ")"
                run(volume_command(image, PROJECT + "_uploads", "/source", "node", "-e", script, writable=True))
        # Retain a separate clean recovery point after removing synthetic probes.
        clean = snapshot(root)
        restore_check(root, clean)
        print("Clean staging snapshot restored and verified:", clean, flush=True)


if __name__ == "__main__":
    try:
        accept(Path("/opt/isaber-staging"))
    except Exception as error:
        print(f"Staging recovery acceptance failed ({type(error).__name__}); raw output omitted")
        raise SystemExit(1)
