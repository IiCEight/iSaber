# ADR 0013: Private HTTPS staging through the existing SSH connection

Status: Implemented on JD for synthetic-content acceptance; configuration is in the staging milestone PR. Owner explicitly chose private HTTPS through SSH.

## Context and decision

JD's public web ports serve an existing relay. The blog also requires HTTPS for secure production cookies and exact-origin authentication. Bind only `127.0.0.1:3443` and use the existing SSH identity to forward the same local port. Caddy's internal CA serves `https://localhost:3443`; the owner imports only its public CA certificate. No DNS, public ingress, existing web listener, or automatic client trust-store change is needed.

Keep a dedicated Compose project with persistent database, uploads, certificate, and proxy-config volumes. Separate the database, application, and edge networks; publish neither the database nor the application HTTP ports. Resource and log limits account for JD's 2 GiB capacity and existing workloads.

Use verified TCR release digests for application/database images. Deliver the official proxy image once through the working offline SSH path, verify its configuration digest, and execute its immutable local Docker engine ID. This avoids another registry setup while retaining a reviewed source/configuration pin. Docker 29's engine ID may differ from the config digest, so derive and check both correctly.

Generate dedicated staging secrets only on the server. The API runtime role receives application DML but no DDL, owner creation/password changes, or migration bookkeeping writes. Explicit owner/migration tools use the operator connection. The constrained owner singleton column receives UPDATE for the existing login query's row lock; password fields remain protected.

## Operational boundary

The initial deployment helper validates schema/platform/repository digests, explicitly migrates, checks private HTTPS, and records completion only after readiness. It preserves secrets and data after failures and refuses a subsequent data-bearing deployment until backup/promotion/rollback is implemented and reviewed.

This is a concrete first cloud staging deployment with restart/authentication acceptance. It is operator-driven, supports synthetic content, and does not imply automatic CD or production recovery readiness. Coordinated backup/restore, promotion/rollback, restricted deployment identity, public ingress, private COS, and final visual design remain subsequent work. See the [staging runbook](../../operations/private-staging.md).
