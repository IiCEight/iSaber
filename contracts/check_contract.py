"""Validate the proposed contract without starting an application server."""

import json
import re
from pathlib import Path

import yaml
from jsonschema import Draft202012Validator, FormatChecker
from openapi_spec_validator import validate_spec


class UniqueKeyLoader(yaml.SafeLoader):
    """Reject duplicate YAML keys instead of silently replacing an endpoint."""


def unique_mapping(loader, node, deep=False):
    result = {}
    for key_node, value_node in node.value:
        key = loader.construct_object(key_node, deep=deep)
        if key in result:
            raise ValueError(f"Duplicate YAML key: {key}")
        result[key] = loader.construct_object(value_node, deep=deep)
    return result


UniqueKeyLoader.add_constructor(
    yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG, unique_mapping
)
root = Path(__file__).resolve().parent
spec = yaml.load((root / "openapi.yaml").read_text(), Loader=UniqueKeyLoader)
validate_spec(spec)


def check_refs(value):
    if isinstance(value, dict):
        if "$ref" in value:
            pointer = value["$ref"]
            if not pointer.startswith("#/"):
                raise ValueError(f"Only local references are allowed: {pointer}")
            target = spec
            for part in pointer[2:].split("/"):
                target = target[part.replace("~1", "/").replace("~0", "~")]
        for child in value.values():
            check_refs(child)
    elif isinstance(value, list):
        for child in value:
            check_refs(child)


check_refs(spec)
operation_ids = set()
operations = 0
for path, methods in spec["paths"].items():
    for method, operation in methods.items():
        if method not in {"get", "post", "patch", "put", "delete", "head", "options"}:
            continue
        operations += 1
        op_id = operation["operationId"]
        assert op_id not in operation_ids, f"Duplicate operationId: {op_id}"
        operation_ids.add(op_id)
        parameters = [
            spec["components"]["parameters"][p["$ref"].split("/")[-1]]
            if "$ref" in p else p
            for p in operation.get("parameters", [])
        ]
        declared = {p["name"] for p in parameters if p["in"] == "path" and p["required"]}
        assert declared == set(re.findall(r"\{([^}]+)\}", path)), path
        unsafe = method not in {"get", "head", "options"}
        if unsafe:
            assert any(p["name"] == "Origin" and p["required"] for p in parameters), path
        if path.startswith("/admin/") or path in {"/auth/session", "/auth/logout"}:
            security = operation.get("security", [])
            assert security and all("ownerSession" in item for item in security), path
            if unsafe:
                assert all("csrfToken" in item for item in security), path


def validator(name):
    return Draft202012Validator(
        {"$ref": f"#/components/schemas/{name}", "components": spec["components"]},
        format_checker=FormatChecker(),
    )


fixtures = json.loads((root / "examples.json").read_text())
for fixture in fixtures:
    name, value = fixture["schema"], fixture["value"]
    validator(name).validate(value)
    assert value in spec["components"]["schemas"][name].get("examples", []), name
for name, schema in spec["components"]["schemas"].items():
    for example in schema.get("examples", []):
        validator(name).validate(example)

invalid = [
    ("DraftPatch", {"expected_version": 1}),
    ("ArticleControls", {"expected_version": 0, "visibility": "public"}),
    ("ArticleControls", {"expected_version": 1, "visibility": "members"}),
    ("SubmitComment", {"display_name": "Guest", "body": "   "}),
    ("SubmitComment", {"display_name": "Guest", "body": "x" * 5001}),
    ("DraftPatch", {"expected_version": 1, "written_at": "2026-02-30"}),
]
for name, value in invalid:
    assert not validator(name).is_valid(value), f"Invalid {name} unexpectedly accepted"

upload = spec["paths"]["/admin/imports"]["post"]["requestBody"]["content"]["multipart/form-data"]["schema"]
assert not Draft202012Validator(upload).is_valid({"file": "fixture", "expected_version": 1})
print(
    f"Contract valid: {operations} operations, {len(spec['components']['schemas'])} schemas, "
    f"{len(fixtures)} fixtures, {len(invalid) + 1} invalid-request checks; "
    "local references and declared owner security checked."
)
