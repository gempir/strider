---
title: Checks
description: Run Strider's unified formatting, maintainability, and correctness checks.
---

`strider check` is the single read-only diagnostic command. It combines
formatting drift, source-level policy, and package-aware correctness findings in
one globally sorted report. Files and parsed source are shared across selected
checks, while more expensive program information is prepared only when a
selected check needs it.

Syntax trees, type information, and control-flow representations are internal
scheduling capabilities. They do not create separate commands, configuration
sections, or rule namespaces, and a check can change implementation without
changing its public code.

## Run checks

Run the configured profile. Its default warning floor runs 96 checks:

```sh
strider check [PATH]...
```

Run the complete 227-check catalog, including notes:

```sh
strider check --all --minimum-severity note [PATH]...
```

Run exactly a subset of codes:

```sh
strider check --only format --only no-init --only invalid-regexp [PATH]...
```

Comma-separated codes are equivalent:

```sh
strider check --only format,no-init,invalid-regexp [PATH]...
```

Explicit selection is case-insensitive and overrides configured `enabled`
states while retaining configured severities, the minimum-severity threshold,
and path exclusions. It also lets Strider avoid preparing capabilities the
selected checks do not need.

Run only checks whose effective severity is warning or higher:

```sh
strider check --minimum-severity warning ./...
```

Selection happens first, then per-rule severity overrides, then the minimum
threshold (`note < warning < error`). `--only` and `--all` do not bypass the
threshold. Checks filtered this way are omitted before Strider plans CST, type,
or SSA work.

Keep an incremental text-mode session open while editing:

```sh
strider check --watch ./...
```

Watch mode reports the initial generation, then emits a fresh complete report
when selected source or its package boundary changes. It reuses unchanged CST
results and only accepts cached package findings after confirming the complete
analysis fingerprint. Baseline generation and pruning, JSON, and HTML are
one-shot operations and cannot be combined with `--watch`.

## Discover checks

Inspect the enabled profile or the complete catalog:

```sh
strider check --list-checks
strider check --all --list-checks
strider check --explain invalid-regexp
```

Strider includes one reserved `format` check, 116 style and maintainability
checks, and 110 correctness and data-flow checks. The reference groups them by
purpose for browsing:

- [Style and maintainability checks](/lints/)
- [Correctness and safety checks](/analyzers/)

## Configure checks

Every check code accepts `enabled`, `severity`, and path `excludes` under the
version-1 `[checks.rules]` namespace:

```toml
version = 1

[checks]
minimum-severity = "warning"

[checks.rules.format]
enabled = true
severity = "note"

[checks.rules.line-length-limit]
enabled = true
severity = "error"
excludes = ["testdata/golden/**"]

[checks.rules.possible-nil-dereference]
severity = "error"
```

The tool-wide minimum severity, exclusions, and default baseline live under
`[checks]`. Formatter layout and formatter-only exclusions remain under
`[formatter]`. See
[Configuration](/configuration/#checks) for the complete contract.

## Formatting findings

Formatting participates as code `format`. An unformatted file produces a normal
note at the start of the file and suggests the write-focused formatter:

```text
note[format]: file is not formatted
  ┌─ main.go:1:1
  = help: run `strider fmt main.go`
```

`check` never writes the formatted candidate. Use `strider fmt`, or
`strider fmt --diff` to inspect the change. Formatting findings are not captured
by baselines.

## Reports

Text is the default human-readable report format:

```text
note[no-init]: replace init with explicit initialization
  ┌─ main.go:12:1
  │
12 │ func init() {
   │ ^^^^^^^^^^^^^

found 1 issue: 1 note
```

Use JSON for integrations or HTML for a self-contained artifact:

```sh
strider check --format json ./...
strider check --format html ./... > check-report.html
```

Each JSON diagnostic includes `code`, `message`, `severity`, `file`, `start`,
and `end`; diagnostics may also carry notes and suggested remedies. HTML reports
include severity totals, search, filters, and source context without external
assets or timestamps.

## Suppressions

Source-local checks support directives on the next declaration or statement:

```go
//strider:ignore no-package-var
var registry = newRegistry()
```

Suppress supported checks for a whole file by placing the directive before
`package`:

```go
//strider:ignore-file no-package-var,no-init
package legacy
```

Use `all` in place of a code to suppress all checks that participate in
source-local suppression at that location. Checks that reason across packages
currently use per-rule exclusions or a baseline instead.

Suppressions should document intentional exceptions. Strider does not yet
report stale or unused suppression codes.
