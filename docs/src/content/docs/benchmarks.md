---
title: Open-source benchmark corpus
description: Deterministic compatibility and performance checks across popular Go projects.
---

Strider continuously formats, lints, and analyzes 10 pinned open-source Go
projects. The corpus protects three contracts:

- no formatter, linter, package-loading, or analyzer processing errors;
- identical exit codes and diagnostic output on every run;
- format, lint, and analysis times remain within reviewed budgets.

[Open the latest reviewed HTML report](/benchmark-report/).

The current corpus includes chi, Gin, gorilla/mux, fsnotify, google/uuid,
Masterminds/semver, shopspring/decimal, gorilla/websocket, fatih/color, and
go-retryablehttp. Every repository is pinned to a full commit SHA in
`benchmarks/projects.json`, so upstream changes cannot silently move the test
target.

## Reproduce locally

From the repository root:

```sh
make corpus-check
```

Projects are cached in `.benchmark-cache/`. Machine-readable and standalone
HTML reports are written to `target/corpus/`. The report lists findings and
elapsed time for every project/operation pair, alongside its budget and baseline
status.

See `CONTRIBUTING.md` for the review process used when results intentionally
change.
