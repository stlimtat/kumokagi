---
title: "ADR 0001: No In-Memory Secret Cache"
weight: 1
---

# ADR 0001: No In-Memory Secret Cache

## Status
Accepted

## Context
The library fetches Secrets from the backend on demand. A naive implementation would cache fetched values in memory to reduce backend API calls. Most secrets libraries (AWS SDK, Azure SDK helpers) cache by default.

## Decision
kumokagi does not cache Secret values. Every fetch call hits the backend.

## Rationale
- Rotation takes effect immediately on the next fetch — no TTL lag, no stale value window
- Cache invalidation on rotation requires either polling or a push signal, both of which add complexity and backend-specific machinery
- The fetch frequency is comparable to other SDK initialisation calls (DB drivers, HTTP clients) — not a hot-path concern
- The Rotation Pattern (reconnect-on-auth-failure) relies on the next fetch returning the rotated value; a cache would silently break this

## Consequences
- Applications must not call `secrets.get()` on every request in a hot path; they should fetch at connection/client initialisation
- No refresh method needed — callers simply fetch again
- Slightly higher backend API call rate than cached alternatives; within normal SDK usage limits
