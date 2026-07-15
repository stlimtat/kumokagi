---
name: Bug report
about: Something isn't working the way the docs say it should
title: "[bug] "
labels: bug
assignees: ""
---

<!--
Before filing: please DO NOT paste secret values, tokens, VAULT_TOKEN, IAM
keys, or `op` session tokens. .kumokagi.yaml itself is safe to share (it holds
no secrets), but redact private Vault/Key Vault URLs if you'd rather not.

Security vulnerabilities: do NOT open a public issue — see SECURITY.md.
-->

## What happened

<!-- A clear description of the bug and what you expected instead. -->

## Environment

- kumokagi version / commit: <!-- `kumokagi version`, git SHA, or module version -->
- Language & version: <!-- Go 1.26 / Python 3.12 -->
- Backend: <!-- vault | aws | azure | gcp | onepassword -->
- Used via: <!-- CLI | Go library (vipersource) | Python library (pydantic-settings) -->
- OS / arch: <!-- e.g. linux/amd64, EKS, local macOS -->
- Auth mechanism: <!-- IRSA | Workload Identity | VAULT_TOKEN | op signin | env vars | local login -->

## `.kumokagi.yaml`

<!-- Paste your config (it contains no secret values). Redact private hostnames if needed. -->

```yaml

```

## Steps to reproduce

1.
2.
3.

## What you expected

<!-- e.g. "kumokagi get db_password prints the value" -->

## What actually happened

<!-- Exact error output. Redact any token/credential that appears in logs. -->

```

```

## Troubleshooting already tried

<!-- Which steps from the backend's How-To "Verify and troubleshoot access"
section did you run, and where did it first fail? e.g. `vault token capabilities`
returned deny; `aws sts get-caller-identity` succeeded. -->
