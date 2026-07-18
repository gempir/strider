---
title: Getting started
description: Build Strider, check a project, and format Go source.
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
print-width = 180

[checks]
minimum-severity = "warning"

[checks.rules.line-length-limit]
enabled = true

[checks.rules.possible-nil-dereference]
severity = "error"
```

See [Configuration](/configuration/) for every formatter, check, path, and
baseline setting.

## Check a project

Run the configured check profile recursively from the current directory:

```sh
strider check
```

The built-in profile selects 118 checks. Its default warning floor runs 96;
select individual codes when investigating a finding or adopting Strider
incrementally:

```sh
strider check --only format,no-init,invalid-regexp ./...
```

Use `--minimum-severity warning` or `--minimum-severity error` to run only the
corresponding policy layers without changing individual rules.

Enable the complete 227-check catalog, including notes, with:

```sh
strider check --all --minimum-severity note ./...
```

`check` is read-only. If the `format` check reports a file, format it with:

```sh
strider fmt path/to/file.go
```

## Format a project

Run the formatter without paths to recursively write the current directory:

```sh
strider fmt
```

Inspect formatting changes without writing them:

```sh
strider fmt --diff
```

## Adopt existing findings

If an established repository has a backlog, generate one check baseline.
Existing matches are suppressed while new findings remain visible:

```sh
strider check --generate-baseline --baseline strider-baseline.toml ./...
```

Formatting findings are not captured. Commit the baseline and configure its
path under `[checks]`. See [Baselines](/baselines/) before regenerating or
pruning it.

## Exit status

| Code | Meaning |
| --- | --- |
| `0` | The command succeeded with no visible findings, or a baseline update completed. |
| `1` | One or more visible check findings were reported. |
| `2` | A command, parsing, package-loading, unsupported-syntax, configuration, baseline, or I/O error occurred. |

Reports and formatted source are written to standard output. Operational errors
are written to standard error.
