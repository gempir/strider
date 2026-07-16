---
title: Getting started
description: Build Strider and run its formatter, linter, and analyzer.
---

## Build

Strider currently targets Go 1.26 and builds as a statically linked binary.

```sh
make build
```

The binary is written to `./strider`. The equivalent Go command is:

```sh
CGO_ENABLED=0 go build -trimpath -o strider ./cmd/strider
```

## Add project configuration

Create `strider.toml` at the repository root. Strider discovers it from the
current directory or any parent:

```toml
version = 1

[formatter]
print-width = 100
max-empty-lines = 1

[linter.rules.line-length-limit]
enabled = true

[analyzer.rules.possible-nil-dereference]
severity = "error"
```

See [Configuration](/configuration/) for every formatter, tool, rule, path,
and baseline setting.

## Format a project

Run the formatter without paths to recursively format the current directory:

```sh
strider fmt
```

Use check mode in CI to report differences without writing:

```sh
strider fmt --check
```

## Lint a project

Run every enabled rule recursively from the current directory:

```sh
strider lint
```

Limit a run to one or more rule codes when investigating or adopting Strider:

```sh
strider lint --only no-init,no-package-var ./...
```

## Analyze a project

Run the package-aware checks, including type checking and SSA construction:

```sh
strider analyze ./...
```

Select one analyzer while investigating a finding:

```sh
strider analyze --only invalid-regexp ./...
```

## Adopt existing findings

If an established repository has a backlog, generate separate lint and
analysis baselines. Existing matches are suppressed while new findings remain
visible:

```sh
strider lint --generate-baseline --baseline lint-baseline.toml ./...
strider analyze --generate-baseline --baseline analysis-baseline.toml ./...
```

Commit the files and configure their paths. See [Baselines](/baselines/) before
regenerating or pruning them.

## Exit status

| Code | Meaning |
| --- | --- |
| `0` | The command succeeded with no findings or formatting differences. |
| `1` | Lint or analysis findings, or formatting differences were found. |
| `2` | A command, parsing, unsupported-syntax, configuration, or I/O error occurred. |

Source output is written to standard output. Operational errors are written to
standard error.
