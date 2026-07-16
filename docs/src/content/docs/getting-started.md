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

## Exit status

| Code | Meaning |
| --- | --- |
| `0` | The command succeeded with no findings or formatting differences. |
| `1` | Lint or analysis findings, or formatting differences were found. |
| `2` | A command, parsing, unsupported-syntax, configuration, or I/O error occurred. |

Source output is written to standard output. Operational errors are written to
standard error.
