---
title: Getting started
description: Build Strider, check a project, and format Go source.
---

Strider is intentionally strict out of the box: its defaults are designed to
surface a broad range of correctness, maintainability, and style issues from
the first run. If that policy is stricter than your project needs, see
[Configuration](/configuration/) to tune the checks and adopt Strider at your
own pace.

## Download

Download and unpack the latest nightly binary for your Linux or macOS machine:

```bash
set -euo pipefail

case "$(uname -s)-$(uname -m)" in
  Darwin-arm64)  asset="strider-nightly-darwin-arm64.tar.gz" ;;
  Darwin-x86_64) asset="strider-nightly-darwin-amd64.tar.gz" ;;
  Linux-aarch64) asset="strider-nightly-linux-arm64.tar.gz" ;;
  Linux-x86_64)  asset="strider-nightly-linux-amd64.tar.gz" ;;
  *) echo "Unsupported platform: $(uname -s)-$(uname -m)" >&2; exit 1 ;;
esac

url="https://github.com/gempir/strider/releases/download/nightly/$asset"
curl -fL "$url" -o "/tmp/$asset"
tar -xzf "/tmp/$asset"
rm "/tmp/$asset"
sudo mv strider /usr/local/bin/strider
```

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

[check]
minimum-severity = "warning"

[checks.file-length-limit]
severity = "warning"
max-lines = 800

[checks.unclosed-http-response-body]
severity = "error"
```

See [Configuration](/configuration/) for every formatter, check, path, and
baseline setting.

## Check a project

Run all checks admitted by the configured severity floor recursively from the
current directory:

```sh
strider check
```

The default warning floor runs warning and error checks. Select individual
codes when investigating a finding or adopting Strider incrementally:

```sh
strider check --only format,no-init,invalid-regexp ./...
```

Use `--minimum-severity warning` or `--minimum-severity error` to run only the
corresponding policy layers without changing individual rules.

Include all note, warning, and error checks with:

```sh
strider check --minimum-severity note ./...
```

`check` is read-only by default. Apply the initial safe automatic fixes for
formatting, double negation, redundant switch breaks, and single-argument
`append` calls with:

```sh
strider check --fix ./...
```

Use `--fix-unsafe` only when you also want fixes classified as potentially
unsafe or unsafe. Both modes rerun the checks and report what remains. See
[Checks](/checks/#apply-automatic-fixes) for selection, validation, and safety
details.

If the `format` check reports a file, the focused formatting command is:

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
path under `[check]`. See [Baselines](/baselines/) before regenerating or
pruning it.

## Exit status

| Code | Meaning |
| --- | --- |
| `0` | The command succeeded with no visible findings after any requested fixes, or a baseline update completed. |
| `1` | One or more visible check findings remain. |
| `2` | A command, fix validation, stale-source, parsing, package-loading, unsupported-syntax, configuration, baseline, or I/O error occurred. |

Reports and formatted source are written to standard output. Operational errors
are written to standard error.
