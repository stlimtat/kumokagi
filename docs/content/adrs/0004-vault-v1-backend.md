---
title: "ADR 0004: HashiCorp Vault as v1 Backend"
weight: 4
---

# ADR 0004: HashiCorp Vault as v1 Backend

## Status
Accepted

## Context
kumokagi targets multiple Secrets Backends (AWS Secrets Manager, Azure Key Vault, GCP Secret Manager, HashiCorp Vault). All backends must be supported eventually via a common Provider interface. One backend must be implemented first to validate the interface.

## Decision
HashiCorp Vault is the v1 Provider implementation. The Provider interface is designed and validated against Vault before any other backend is added.

## Rationale
Vault covers the widest surface area of the requirements: human auth via LDAP/OIDC, CI auth via JWT/OIDC auth method, K8s workload auth via Kubernetes auth method, KV v2 for secret versioning, and configurable mount points. Validating the interface against the most capable backend reduces the risk of the interface being too narrow for managed cloud backends to implement cleanly.

The immediate operational pain (Azure Key Vault → GitHub → 1Password chain) is also addressed by Vault, which was the triggering use case.

## Consequences
- v1 ships one Provider; AWS/Azure/GCP Providers follow the same interface
- The Provider interface must not assume Vault-specific features (dynamic secrets, leases) that managed backends lack
- Vault Agent and Vault Secrets Operator setup is documented as a howto guide; it is out of scope for the library
