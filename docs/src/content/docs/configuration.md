---
title: Configuration
description: Configure Strider's formatter, unified checks, exclusions, severities, and baseline.
---

Strider reads project settings from one strict TOML file named `strider.toml`.
Version 1 uses `[check]` for command-wide policy and `[checks.<code>]` for
formatting, maintainability, and correctness diagnostics, while `[formatter]`
controls rendered source.

```toml
version = 1
color = "auto"

[formatter]
print-width = 180
excludes = ["internal/generated/**"]

[check]
excludes = ["testdata/**"]
baseline = "strider-baseline.toml"
minimum-severity = "warning"

[checks.format]
severity = "warning"

[checks.no-init]
severity = "none"

[checks.file-length-limit]
severity = "error"
max-lines = 800
excludes = ["cmd/migrations/**"]

[checks.possible-nil-dereference]
severity = "error"
```

Only settings that differ from Strider's defaults are required.

## Discovery

Without a global option, Strider starts in the current working directory and
looks for `strider.toml`. If it is absent, Strider checks each parent directory
up to the filesystem root. The nearest file wins, so commands launched from a
nested package still use the repository policy.

Choose a file explicitly when testing another policy:

```sh
strider --config configs/strict.toml check ./...
```

Temporarily use only built-in defaults:

```sh
strider --no-config check ./...
```

`--config` and `--no-config` are global options, so they must appear before the
command. They are mutually exclusive. An explicit path must exist; automatic
discovery quietly falls back to defaults when no file exists.

## Version and strict validation

The configuration version is `1`; new files should declare it explicitly.
There is no alternate or legacy schema.

Strider rejects malformed TOML, unsupported versions, unknown section keys,
unknown check names, invalid severities, and formatter values outside their
accepted ranges. A configuration error exits with status `2` before source
files are changed or diagnostics are reported.

This strictness catches misspellings such as `severty = "error"` instead of
silently running with a different policy.

Strider does not currently support inherited configuration, environment
variable overrides, user-wide configuration, or a published JSON Schema.

## Precedence

Effective behavior is assembled in this order:

1. Built-in defaults.
2. The discovered or explicitly selected `strider.toml`.
3. Command-line selection, severity, and baseline flags.

`check --only CODE` narrows the catalog but still applies configured severity
and per-rule exclusions. To investigate checks configured with
`severity = "none"`, use `--minimum-severity none`, optionally together with
`--only`.

An explicit `--baseline PATH` overrides `[check].baseline`. The global
`--color` option similarly overrides the top-level `color` setting.

## Terminal output

The top-level `color` setting controls ANSI styling for human-readable output.

| Setting | Type | Default | Accepted values and effect |
| --- | --- | --- | --- |
| `color` | string | `"auto"` | `"auto"` uses color for terminals, `"always"` forces it, and `"never"` disables it. |

Strider applies semantic colors throughout its human interface: errors are red,
warnings yellow, notes blue, and check codes, paths, source gutters, commands,
and options use xterm color 2. Successful output, suggested remedies, and diff
additions use xterm color 10; diff removals remain red. JSON and formatter
standard-input source are never styled.

For one run, place the global override before the command:

```sh
strider --color always check ./...
strider --color never fmt ./...
```

Strider also honors the `NO_COLOR` and `FORCE_COLOR` conventions. A non-empty
`FORCE_COLOR` has precedence over `NO_COLOR` and the configured or CLI mode;
`FORCE_COLOR=0` explicitly disables color.

## Path patterns

`excludes` is supported by the formatter, the unified check runner, and every
individual check. Paths are evaluated relative to the directory containing
`strider.toml`.

- A plain path such as `testdata` or `internal/generated` matches that path and
  everything below it.
- A glob may use `*`, `?`, character classes such as `[a-z]`, and `**` for
  recursive directory matching.
- Paths use slash-separated project-relative spelling on every operating
  system.

```toml
[formatter]
excludes = ["vendor-tools", "**/*.generated.go"]

[check]
excludes = ["testdata/**"]

[checks.package-comments]
excludes = ["cmd/**", "examples/**"]
```

`[check].excludes` suppresses diagnostics and fixes for matching files across
all checks. Those files remain part of workspace discovery and package loading,
so their declarations can still affect package-level analysis of other files.
Per-check exclusions suppress only that code. `[formatter].excludes` applies
to the `fmt` command and to code `format`; it does not apply to formatter stdin,
which has no discovery pass.

## Formatter

Formatter settings live under `[formatter]`.

| Setting | Type | Default | Accepted values and effect |
| --- | --- | --- | --- |
| `print-width` | integer | `180` | Wrap target from `40` through `500` columns. |
| `excludes` | string list | `[]` | Plain paths or globs skipped by filesystem formatting and the `format` check. |

The formatter remains intentionally opinionated. Imports are sorted into
standard-library, third-party, and current-module groups; indentation uses
tabs; broken lists use trailing commas; top-level declarations use const, var,
type, then func order; binary operators remain on the preceding line; and output
has exactly one final newline. A whole-file `strider:format-ignore` directive is
recognized only in comments before the package clause.

## Checks

Tool-wide settings live under `[check]`.

| Setting | Type | Default | Effect |
| --- | --- | --- | --- |
| `excludes` | string list | `[]` | Suppress diagnostics and fixes in matching files for all checks while retaining them for package analysis. |
| `baseline` | string | unset | Apply this baseline unless the CLI overrides or ignores it. Relative paths resolve from `strider.toml`. |
| `minimum-severity` | string | `"warning"` | Run only checks whose effective severity is at least `"none"`, `"note"`, `"warning"`, or `"error"`. |
All 207 checks are eligible. The default warning floor runs the 191 checks whose
effective severity is warning or error. `strider check --minimum-severity note`
also runs note checks, while `strider check --minimum-severity none` additionally
runs checks configured with severity `none`.

Every code accepts the same two common options:

| Setting | Type | Default | Effect |
| --- | --- | --- | --- |
| `severity` | string | Check default | Report as `"none"`, `"note"`, `"warning"`, or `"error"`; `"none"` suppresses the check at ordinary minimum severities. Every reported finding still exits `1`. |
| `excludes` | string list | `[]` | Skip only this check on matching paths. |

```toml
[checks.format]
severity = "warning"

[checks.file-length-limit]
severity = "warning"
max-lines = 800
excludes = ["testdata/golden/**"]

[checks.invalid-regexp]
severity = "none"
```

Eight checks accept additional behavioral options. A numeric value of `0`
uses the built-in limit, except for `file-length-limit`, where `0` disables the
check. Empty lists are meaningful: they allow all characters or block no
imports.

| Check | Setting | Type | Built-in behavior |
| --- | --- | --- | --- |
| [`banned-characters`](/lints/banned-characters/) | `characters` | string list | Ban `ᐸ` and `ᐳ`; `[]` bans none. |
| [`file-length-limit`](/lints/file-length-limit/) | `max-lines` | integer | 500; `0` disables the check. |
| [`function-length`](/lints/function-length/) | `max-lines`, `max-statements` | integer | 75 lines and 50 statements; `0` uses either built-in limit. |
| [`function-result-limit`](/lints/function-result-limit/) | `max-results` | integer | 3; `0` uses the built-in limit. |
| [`imports-blocklist`](/lints/imports-blocklist/) | `blocked-imports` | string list | `[]` blocks no imports. |
| [`max-parameters`](/lints/max-parameters/) | `max-parameters` | integer | 8; `0` uses the built-in limit. |
| [`max-public-structs`](/lints/max-public-structs/) | `max-public-structs` | integer | 5; `0` uses the built-in limit. |
| [`interface-method-limit`](/analyzers/interface-method-limit/) | `max-methods` | integer | 10; `0` uses the built-in limit. |

Each linked check page shows its complete TOML table and examples. Check-specific
options are rejected on unrelated check codes.

Strider resolves selection and each rule's severity before applying
`minimum-severity`, using `none < note < warning < error`. A note promoted to error is
therefore included in an error-only run; an error demoted to note is excluded
from a warning-only run. A check set to none is omitted unless the minimum is
explicitly lowered to none. Explicit `--only` selection still respects the
minimum. Override it for one command with `--minimum-severity`.

Strider may satisfy selected checks from source text, syntax, type information,
or control-flow data. These capabilities are internal scheduling details: they
are not configuration categories, and Strider prepares only the union required
by the selected codes.

## Baseline path

Configure one baseline for the unified diagnostic set:

```toml
[check]
baseline = "strider-baseline.toml"
```

Relative baseline paths resolve from the configuration directory, not the
shell's current directory. The `format` check is never stored in a baseline.
See [Baselines](/baselines/) for generation, matching, pruning, and CI adoption.

## Source directives

Configuration controls project policy; source directives record local
exceptions. See [Suppress checks](/suppress/) for inline, file-level, and
formatter directives.
