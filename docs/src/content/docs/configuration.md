---
title: Configuration
description: Configure Strider's formatter, unified checks, exclusions, severities, and baseline.
---

Strider reads project settings from one strict TOML file named `strider.toml`.
Version 2 has one `[checks]` namespace for formatting, maintainability, and
correctness diagnostics, while `[formatter]` controls rendered source.

```toml
version = 2
color = "auto"

[formatter]
print-width = 100
indent-width = 4
max-empty-lines = 1
end-of-line = "lf"
excludes = ["internal/generated/**"]

[checks]
excludes = ["testdata/**"]
baseline = "strider-baseline.toml"
baseline-variant = "loose"

[checks.rules.format]
severity = "warning"

[checks.rules.no-init]
enabled = false

[checks.rules.line-length-limit]
enabled = true
severity = "error"
excludes = ["cmd/migrations/**"]

[checks.rules.possible-nil-dereference]
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

The canonical configuration version is `2`. New files should declare it
explicitly. Earlier version-1 files remain readable for compatibility, but a
document cannot mix sections from different versions.

Strider rejects malformed TOML, unsupported versions, unknown section keys,
unknown check names, invalid severities, invalid baseline variants, and
formatter values outside their accepted ranges. A configuration error exits
with status `2` before source files are changed or diagnostics are reported.

This strictness catches misspellings such as `severty = "error"` instead of
silently running with a different policy.

Strider does not currently support inherited configuration, environment
variable overrides, user-wide configuration, or a published JSON Schema.

## Precedence

Effective behavior is assembled in this order:

1. Built-in defaults.
2. The discovered or explicitly selected `strider.toml`.
3. Command-line selection and baseline flags.

`check --only CODE` and `check --all` enable requested checks even when their
configuration says `enabled = false`. Configured severity and per-rule
exclusions still apply. This makes `--only` useful for investigating a disabled
check without editing the project file.

An explicit `--baseline PATH` overrides `[checks].baseline`.
`--baseline-variant` overrides the configured variant for generation. The
global `--color` option similarly overrides the top-level `color` setting.

## Terminal output

The top-level `color` setting controls ANSI styling for human-readable output.

| Setting | Type | Default | Accepted values and effect |
| --- | --- | --- | --- |
| `color` | string | `"auto"` | `"auto"` uses color for terminals, `"always"` forces it, and `"never"` disables it. |

Strider applies semantic colors throughout its human interface: errors are red,
warnings yellow, notes blue, check codes magenta, paths and source gutters cyan,
suggested remedies green, and diff removals red. JSON and formatter
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

[checks]
excludes = ["testdata/**"]

[checks.rules.package-comments]
excludes = ["cmd/**", "examples/**"]
```

`[checks].excludes` removes matching files from the entire diagnostic run.
Per-check exclusions disable only that code. `[formatter].excludes` applies to
the `fmt` command and to code `format`; it does not apply to formatter stdin,
which has no discovery pass.

## Formatter

Formatter settings live under `[formatter]`.

| Setting | Type | Default | Accepted values and effect |
| --- | --- | --- | --- |
| `print-width` | integer | `100` | Wrap target from `40` through `500` columns. |
| `indent-width` | integer | `4` | Display width of an indentation tab, from `1` through `16`; output indentation remains tabs. |
| `max-empty-lines` | integer | `1` | Preserve at most this many consecutive empty lines; any nonnegative value is accepted. |
| `end-of-line` | string | `"lf"` | `"lf"` or `"crlf"`. |
| `excludes` | string list | `[]` | Plain paths or globs skipped by filesystem formatting and the `format` check. |

The formatter remains intentionally opinionated. Imports are sorted into
standard-library, third-party, and current-module groups; indentation uses
tabs; broken lists use trailing commas; binary operators remain on the
preceding line; and output has exactly one final newline.

## Checks

Tool-wide settings live under `[checks]`.

| Setting | Type | Default | Effect |
| --- | --- | --- | --- |
| `excludes` | string list | `[]` | Skip matching files for all checks. |
| `baseline` | string | unset | Apply this baseline unless the CLI overrides or ignores it. Relative paths resolve from `strider.toml`. |
| `baseline-variant` | string | `"loose"` | Shape used the next time a baseline is generated: `"loose"` or `"strict"`. |
| `rules` | table | `{}` | Common configuration keyed by any registered check code. |

The default profile contains 94 checks: `format`, seven style and
maintainability checks, and all 86 package-aware correctness checks. The other
109 style and maintainability checks are optional. `strider check --all` enables
all 203 checks.

Every code accepts the same three options:

| Setting | Type | Default | Effect |
| --- | --- | --- | --- |
| `enabled` | boolean | Check profile default | Include or omit the check during an ordinary configured run. |
| `severity` | string | Check default | Report as `"note"`, `"warning"`, or `"error"`; every visible severity still exits `1`. |
| `excludes` | string list | `[]` | Skip only this check on matching paths. |

```toml
[checks.rules.format]
enabled = true
severity = "warning"

[checks.rules.line-length-limit]
enabled = true
severity = "warning"
excludes = ["testdata/golden/**"]

[checks.rules.invalid-regexp]
enabled = false
```

Changing the severity of an optional check does not implicitly enable it. Set
`enabled = true` as well, or select it on the CLI. Behavioral thresholds remain
part of each check's contract and are not yet generally configurable.

Strider may satisfy selected checks from source text, syntax, type information,
or control-flow data. These capabilities are internal scheduling details: they
are not configuration categories, and Strider prepares only the union required
by the selected codes.

## Baseline path

Configure one baseline for the unified diagnostic set:

```toml
[checks]
baseline = "strider-baseline.toml"
baseline-variant = "loose"
```

Relative baseline paths resolve from the configuration directory, not the
shell's current directory. The `format` check is never stored in a baseline.
See [Baselines](/baselines/) for generation, matching, pruning, backups, and CI
adoption.

## Source directives

Configuration controls project policy; source directives record local
exceptions.

Skip formatting for an entire file:

```go
//strider:format-ignore
```

Suppress supported source-local checks for the next declaration or statement:

```go
//strider:ignore no-package-var,no-init
```

Place a file-level directive before the package clause:

```go
//strider:ignore-file no-package-var
package example
```

Use the special code `all` to suppress every check that participates in
source-local suppression at that location. Other checks use per-rule exclusions
or the baseline.
