# Security Policy

## Reporting a vulnerability

Please report security vulnerabilities **privately**, not via public issues or
pull requests.

- Preferred: open a private advisory at
  <https://github.com/stlimtat/kumokagi/security/advisories/new>.
- Or email the maintainer (see the commit history for the address).

Include a description, affected version/commit, and a minimal reproduction if
you have one. You'll get an acknowledgement, and a fix or mitigation timeline
once the report is triaged.

Please do not include real secrets, tokens, or credentials in your report — a
redacted reproduction is enough.

## Scope and threat model

kumokagi fetches secrets from a backend into process memory using **ambient
credentials only** — it never stores a credential of its own. Two boundaries
are worth stating explicitly:

- **`.kumokagi.yaml` is a trust boundary.** The docs encourage committing it
  because it holds no secret values, but it *does* choose the backend and its
  endpoint (`vault.address`, `azure.vault_url`, `gcp.project`). A malicious
  change to that file can point a backend at an attacker-controlled host and
  exfiltrate the ambient token. Treat a config change as sensitive as a code
  change and review it. For defense in depth, set the fail-closed allowlists:
  `KUMOKAGI_ALLOWED_VAULT_ADDRS`, `KUMOKAGI_ALLOWED_AZURE_VAULTS`,
  `KUMOKAGI_ALLOWED_GCP_PROJECTS`.
- **Ambient credentials are the deployment's responsibility.** kumokagi
  delegates entirely to the backend SDK's default credential chain (IRSA,
  Workload Identity, `VAULT_TOKEN`, `op signin`). Misconfigured IAM/policy is
  out of scope for kumokagi itself; the per-backend
  [How-To guides](https://stlimtat.github.io/kumokagi/how-to/) include a
  verify-and-troubleshoot ladder.

In scope: path/URI/argument injection, credential leakage, unsafe subprocess
or backend calls, and validation bypasses in the library or CLI.

## Supported versions

kumokagi is pre-1.0. Security fixes land on `master`; please test against the
latest commit before reporting.
