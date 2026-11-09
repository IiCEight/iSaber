"""Public, signed manifest format shared by the backup sender and vault."""
from datetime import datetime, timezone
import json
import re

MAX_CIPHER_BYTES = 2 * 1024**3
MAX_PLAIN_BYTES = 4 * 1024**3
FIELDS = {"version", "id", "cipher_bytes", "cipher_sha256", "plain_bytes", "plain_sha256", "backup_sha256", "source_commit", "recipient"}


def validate_id(value):
    if not isinstance(value, str) or not re.fullmatch(r"[0-9]{8}T[0-9]{6}Z-[0-9a-f]{8}", value):
        raise ValueError("Invalid snapshot identifier")
    return datetime.strptime(value.split("-", 1)[0], "%Y%m%dT%H%M%SZ").replace(tzinfo=timezone.utc)


def validate_manifest(value):
    if not isinstance(value, dict) or set(value) != FIELDS or type(value.get("version")) is not int or value["version"] != 1:
        raise ValueError("Unsupported signed backup manifest")
    validate_id(value["id"])
    for name, maximum in (("cipher_bytes", MAX_CIPHER_BYTES), ("plain_bytes", MAX_PLAIN_BYTES)):
        if type(value[name]) is not int or not 0 < value[name] <= maximum:
            raise ValueError("Backup exceeds the size limit")
    for name in ("cipher_sha256", "plain_sha256", "backup_sha256"):
        if not isinstance(value[name], str) or not re.fullmatch(r"[0-9a-f]{64}", value[name]):
            raise ValueError("Invalid backup receipt digest")
    if not isinstance(value["source_commit"], str) or not re.fullmatch(r"[0-9a-f]{40}", value["source_commit"]):
        raise ValueError("Invalid release identity")
    if not isinstance(value["recipient"], str) or not re.fullmatch(r"age1[0-9a-z]{58}", value["recipient"]):
        raise ValueError("Require the owner's dedicated age recipient")
    return value


def canonical(value):
    validate_manifest(value)
    return (json.dumps(value, sort_keys=True, separators=(",", ":")) + "\n").encode()


def validate_signature(value):
    if not isinstance(value, str) or len(value) > 4096 or not re.fullmatch(r"-----BEGIN SSH SIGNATURE-----\n[A-Za-z0-9+/=\n]+-----END SSH SIGNATURE-----\n?", value):
        raise ValueError("Invalid bounded SSH signature")
    return value
