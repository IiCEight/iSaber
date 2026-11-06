# API contract

Status: Draft for review, not an implemented API. Start with the [API design](../docs/architecture/api-design.md) and [data model](../docs/architecture/data-model.md).

- `openapi.yaml`: OpenAPI 3.1.1 contract for the public blog and owner dashboard.
- `examples.json`: synthetic response/request fixtures for review and initial mocks. These do not represent real owner content or credentials.
- `check_contract.py`: validates the specification, local references, examples, selected invalid requests, and declared owner security requirements. It does not test a running server or prove runtime authorization.
- `requirements.txt`: pinned direct dependencies for contract validation; application dependencies remain separate.

## Validate

From the repository root, create an isolated environment outside the repository:

```bash
python3 -m venv /tmp/isaber-contract-venv
/tmp/isaber-contract-venv/bin/python -m pip install -r contracts/requirements.txt
/tmp/isaber-contract-venv/bin/python contracts/check_contract.py
```

The checker resolves local references only. Client generation, server contract testing, and mock-server tooling will be selected after API review. Additive compatibility checks and CI wiring belong to the application foundation milestone.

## Review workflow

1. Review lifecycle and access rules in the architecture documents.
2. Review endpoints and their schemas in `openapi.yaml`.
3. Review representative content in `examples.json`; keep schema examples synchronized.
4. Run validation after changes.
5. Approve the contract before generating application clients or implementing handlers.

The API branch is based on `docs/product-requirements` so it contains the approved product baseline. Review and merge that branch before merging this branch; source changes are not yet published to a Git host.
