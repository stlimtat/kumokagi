---
name: New backend request
about: Request (or propose to contribute) support for another secrets backend
title: "[backend] Support for <name>"
labels: enhancement, backend
assignees: ""
---

## Backend

<!-- Name and link, e.g. Doppler, Infisical, CyberArk, AWS Parameter Store. -->

## Authentication model

<!-- kumokagi uses ambient credentials only — it never stores a credential.
How does a workload authenticate to this backend without a stored secret?
(OIDC federation, instance identity, CLI session, etc.) -->

## Path / naming constraints

<!-- How are secrets addressed? Any charset limits on names? kumokagi maps
{mount}/{env}/{app}/{key} onto each backend (e.g. double-dash for Azure/GCP). -->

## Are you offering to contribute it?

<!-- The provider interface is small (Get/Set/Delete/List/Exists). See
https://stlimtat.github.io/kumokagi/contributing/ — a new backend needs the Go
provider, the Python twin, and a How-To page with an auth diagram. -->

- [ ] Yes, I'd like to implement it
- [ ] No, requesting only
