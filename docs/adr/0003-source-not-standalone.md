# ADR 0003: kumokagi as a Config Source, Not a Standalone Config System

## Status
Accepted

## Context
The library needs to deliver Secret values to application code. Two architectural positions exist:
- **Standalone**: kumokagi owns the config API. Apps call `kumokagi.Get("key")` directly.
- **Source**: kumokagi registers as a provider/source inside existing config frameworks (viper for Go, pydantic-settings for Python). Apps use their existing config framework as normal.

## Decision
kumokagi is a Source:
- **Go**: implements viper's remote provider interface
- **Python**: implements pydantic-settings' `BaseSettings` custom source interface

## Rationale
Applications already use viper and pydantic-settings for non-secret config (feature flags, service URLs, timeouts). A standalone secrets API would require apps to manage two config systems with different APIs and different precedence logic.

As a Source, kumokagi participates in the existing precedence chain — env vars can override secrets backend values in development without code changes. The app has one config API regardless of where values originate.

## Consequences
- kumokagi has a hard dependency on viper (Go) and pydantic-settings (Python) as integration targets
- The library interface is shaped by what viper and pydantic-settings can express — custom features must fit within their extension points
- Adding support for other config frameworks (e.g. envconfig, dynaconf) requires new Source implementations
