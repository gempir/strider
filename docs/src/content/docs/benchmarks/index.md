---
title: Open-source benchmark corpus
description: Deterministic compatibility and performance checks across popular Go projects.
sidebar:
  order: 0
  label: Overview
---

Strider continuously formats, lints, and analyzes 10 pinned open-source Go
projects. The corpus protects three contracts:

- no formatter, linter, package-loading, or analyzer processing errors;
- identical exit codes and diagnostic output on every run;
- format, lint, and analysis times remain within reviewed budgets.

## Latest reviewed results

The report below shows all 30 project/operation results, finding totals, elapsed
times, budgets, and baseline status. Select a project from the **Benchmarks**
sidebar group to browse every finding with highlighted source context.

<iframe
  class="benchmark-report"
  src="/benchmark-report/"
  title="Latest reviewed Strider open-source corpus results"
  loading="lazy"
></iframe>

[Open the overview report as a full page](/benchmark-report/).

## Reproduce locally

```sh
make corpus-check
```

Projects are cached in `.benchmark-cache/`. Machine-readable and standalone
HTML reports are written to `target/corpus/`. See `CONTRIBUTING.md` for the
review process used when results intentionally change.
