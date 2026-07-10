---
title: Contributing
weight: 6
---

# Contributing

Contributions are welcome — bug reports, new providers, documentation fixes, and everything in between. kumokagi is [MIT-licensed]({{< relref "/license" >}}), so your contributions are too.

## Where the code lives

Everything is in the [stlimtat/kumokagi](https://github.com/stlimtat/kumokagi) repository:

| Path | What it is |
|------|------------|
| `pkg/` | Go library — config, factory, providers, vipersource |
| `pkg/providers/<backend>/` | One package per backend; each links only its own SDK |
| `cmd/kumokagi/` | The CLI |
| `python/` | The Python library (pydantic-settings source) |
| `docs/` | This site (Hugo, gruvbox theme) |
| `docs/content/adrs/` | Architecture Decision Records |

## Getting started

```bash
git clone https://github.com/stlimtat/kumokagi
cd kumokagi

# Go: vet and test
go vet ./...
go test -race -count=1 ./...

# Python: install dev deps and test
cd python
pip install -e ".[dev]"
pytest tests/ -v
```

## Making a change

1. **Open an issue first** for anything larger than a typo — especially new backends or interface changes, so we can agree on the approach before you invest time.
2. **Read the [ADRs]({{< relref "/adrs" >}})**. A PR that re-introduces an in-memory cache or stores credentials in config will be declined on ADR grounds, not code quality.
3. Fork, branch from `master`, and keep the diff focused — one concern per PR.
4. Add tests. New provider code needs unit tests against a fake/recorded backend; a change in path encoding needs a table test.
5. Run the checks above; CI runs the same (`go vet`, `go test -race`, `pytest`) plus coverage upload.
6. Open the PR with a short description of *why*, not just *what*.

## Adding a new provider

The provider interface is small:

```go
type Provider interface {
    Get(ctx context.Context, path SecretPath) (string, error)
    Set(ctx context.Context, path SecretPath, value string) error
    Delete(ctx context.Context, path SecretPath) error
    List(ctx context.Context, path SecretPath) ([]string, error)
    Exists(ctx context.Context, path SecretPath) (bool, error)
}
```

A new backend needs:

- `pkg/providers/<name>/` implementing the interface, registering itself in an `init()` (the `database/sql` driver pattern)
- ambient-credential auth only — no credential fields in `.kumokagi.yaml`
- a path encoding that fits the backend's naming rules (see [Providers]({{< relref "/introduction/providers" >}}))
- the Python twin in `python/kumokagi/providers/`
- a how-to page under `docs/content/how-to/` with an auth diagram and a troubleshooting section

## Documentation changes

The site is Hugo with vendored modules:

```bash
cd docs
npm install          # theme assets (Prism, fonts, icons)
hugo server          # http://localhost:1313/kumokagi/
```

Diagrams are generated — do not edit the PNGs by hand:

```bash
pip install pillow
python3 docs/diagrams/generate.py
```
