# API contract

Status: Approved API contract baseline. Owner authentication, publishing/images/history, labels/search (PR #9) and guest comments/moderation (PR #10) are merged and deployed. The contract covers those public/owner workflows; runtime behavior is exercised by separate tests. Private persistent uploads are deployed; COS is optional future storage work. See the [publishing guide](../docs/operations/publishing.md), [API design](../docs/architecture/api-design.md) and [data model](../docs/architecture/data-model.md).

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

The checker resolves local references only. API review is complete; client generation, server contract testing, and mock-server tooling remain to be selected. Additive compatibility checks and CI wiring belong to the application foundation milestone.

## Review workflow

1. Review lifecycle and access rules in the architecture documents.
2. Review endpoints and their schemas in `openapi.yaml`.
3. Review representative content in `examples.json`; keep schema examples synchronized.
4. Run validation after changes.
5. Review subsequent contract changes for compatibility; baseline owner approval is recorded in the architecture documents.

The product baseline and API contract were merged into `main` with the initial CI setup. Subsequent application branches must preserve this reviewed contract or document and review intentional compatibility changes.
