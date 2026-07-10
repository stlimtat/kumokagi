---
title: How-To Guides
weight: 3
---

# How-To Guides

One guide per backend, each covering the full journey: how authentication works (with a diagram), configuring kumokagi, storing and fetching secrets in Go and Python, granting least-privilege access, CI and Kubernetes identity setup, and — most importantly — **how to verify and troubleshoot access** when a pod cannot read its secret.

## Backends

- [HashiCorp Vault]({{< relref "vault" >}}) — self-hosted, the most capable backend
- [AWS Secrets Manager]({{< relref "aws" >}}) — IRSA on EKS, OIDC in CI
- [Azure Key Vault]({{< relref "azure" >}}) — Workload Identity on AKS
- [GCP Secret Manager]({{< relref "gcp" >}}) — Workload Identity Federation on GKE
- [1Password]({{< relref "onepassword" >}}) — zero infrastructure, ideal for solo devs

## Language integrations

- [Go — Cobra/Viper]({{< relref "cobra-viper" >}}) — wiring kumokagi into a cobra CLI's viper chain
- [Python — pydantic-settings]({{< relref "pydantic" >}}) — kumokagi as a custom settings source

## Scenarios

- [Startup MVP Guide]({{< relref "startup-mvp" >}}) — from zero infra (1Password) to production (AWS + IRSA)
