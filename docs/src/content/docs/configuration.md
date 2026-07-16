---
title: Configuration
description: Configure formatter layout, file exclusions, every lint rule, every analyzer, and baselines.
---

Strider reads project settings from one strict TOML file named `strider.toml`.
The file can configure all registered lint rules and analyzers through common
rule options, select the formatter's supported layout settings, exclude paths,
and make lint or analysis baselines automatic.

```toml
version = 1
color = "auto"

[formatter]
print-width = 100
indent-width = 4
max-empty-lines = 1
end-of-line = "lf"
excludes = ["internal/generated/**"]

[linter]
excludes = ["testdata/**"]
baseline = "lint-baseline.toml"
baseline-variant = "loose"

[linter.rules.no-init]
enabled = false

[linter.rules.cyclomatic-complexity]
enabled = true
severity = "error"
excludes = ["cmd/migrations/**"]

[analyzer]
baseline = "analysis-baseline.toml"
baseline-variant = "loose"

[analyzer.rules.invalid-regexp]
enabled = true
severity = "error"
```

Only the settings you want to change are required. Omitted values retain
Strider's built-in defaults.

## Discovery

Without a global option, Strider starts in the current working directory and
looks for `strider.toml`. If it is absent, Strider checks each parent directory
up to the filesystem root. The nearest file wins. This lets commands run from
a nested package while still using the repository configuration.

Choose a file explicitly when testing another policy:

```sh
strider --config configs/strict.toml lint ./...
```

Temporarily use only built-in defaults:

```sh
strider --no-config lint ./...
```

`--config` and `--no-config` are global options, so they must appear before the
command. They are mutually exclusive. An explicit path must exist; automatic
discovery quietly falls back to defaults when no file exists.

## Strict validation

The configuration version is `1`. `version = 1` documents that contract and
is recommended; an omitted version currently defaults to version 1.

Strider rejects malformed TOML, unsupported versions, unknown section keys,
unknown rule names, invalid severities, invalid baseline variants, and
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

CLI rule selection has the final say. `lint --only CODE`, `analyze --only
CODE`, and `lint --all-rules` enable the requested rules even when their
configuration says `enabled = false`. Their configured severity and per-rule
exclusions still apply. This makes `--only` useful for investigating a disabled
rule without editing the project file.

An explicit `--baseline PATH` overrides the baseline path in the relevant tool
section. `--baseline-variant` overrides the configured variant for generation.
The global `--color` option similarly overrides the top-level `color` setting.

## Terminal output

The top-level `color` setting controls ANSI styling for human-readable output.

| Setting | Type | Default | Accepted values and effect |
| --- | --- | --- | --- |
| `color` | string | `"auto"` | `"auto"` uses color for terminals, `"always"` forces it, and `"never"` disables it. |

Strider applies semantic colors throughout its human interface: errors are red,
warnings yellow, notes blue, rule codes magenta, paths and source gutters cyan,
successful examples/additions green, and removals red. Rich lint and analysis
reports include source context and a severity summary. JSON and formatter
standard-input source stay machine-clean.

For one run, place the global override before the command:

```sh
strider --color always lint ./...
strider --color never analyze ./...
```

Strider also honors the `NO_COLOR` and `FORCE_COLOR` conventions. A non-empty
`FORCE_COLOR` has precedence over `NO_COLOR` and the configured/CLI mode;
`FORCE_COLOR=0` explicitly disables color.

## Path patterns

`excludes` is supported by the formatter, linter, analyzer, and every individual
rule. Paths are evaluated relative to the directory containing `strider.toml`.

- A plain path such as `testdata` or `internal/generated` matches that path and
  everything below it.
- A glob may use `*`, `?`, character classes such as `[a-z]`, and `**` for
  recursive directory matching.
- Paths use slash-separated project-relative spelling in configuration, on all
  operating systems.

```toml
[formatter]
excludes = ["vendor-tools", "**/*.generated.go"]

[linter]
excludes = ["testdata/**"]

[linter.rules.package-comments]
excludes = ["cmd/**", "examples/**"]
```

Tool-level exclusions remove a file from that tool's reported results. A
per-rule exclusion disables only that rule for matching files. Formatter
exclusions do not apply to `--stdin`, because standard-input mode has no
discovery pass.

## Formatter

Formatter settings live under `[formatter]`.

| Setting | Type | Default | Accepted values and effect |
| --- | --- | --- | --- |
| `print-width` | integer | `100` | Wrap target from `40` through `500` columns. |
| `indent-width` | integer | `4` | Display width of one indentation tab, from `1` through `16`; affects fit calculations. Output indentation remains tabs. |
| `max-empty-lines` | integer | `1` | Preserve at most this many consecutive empty lines. Use `0` to remove optional empty lines; any nonnegative value is accepted. |
| `end-of-line` | string | `"lf"` | `"lf"` or `"crlf"`. |
| `excludes` | string list | `[]` | Plain paths or globs skipped by filesystem formatting. |

```toml
[formatter]
print-width = 120
indent-width = 8
max-empty-lines = 2
end-of-line = "crlf"
excludes = ["internal/generated/**"]
```

The formatter remains intentionally opinionated. Imports are sorted into
standard-library, third-party, and current-module groups; indentation uses
tabs; broken lists use trailing commas; binary operators remain on the
preceding line; and output has exactly one final newline. Those decisions are
not configurable in version 1.

## Linter

Tool-wide settings live under `[linter]`.

| Setting | Type | Default | Effect |
| --- | --- | --- | --- |
| `excludes` | string list | `[]` | Skip matching files for every lint rule. |
| `baseline` | string | unset | Apply this baseline unless the CLI overrides or ignores it. Relative paths resolve from `strider.toml`. |
| `baseline-variant` | string | `"loose"` | Variant used the next time a baseline is generated: `"loose"` or `"strict"`. |
| `rules` | table | `{}` | Common configuration keyed by any registered lint code. |

With no selection flags and no rule configuration, the seven-rule default
profile is enabled: `cyclomatic-complexity`, `max-parameters`,
`no-naked-return`, `no-init`, `no-package-var`, `no-defer-in-loop`, and
`no-else-after-return`. Other lint rules are disabled until selected by
`--all-rules`, `--only`, or `enabled = true`.

### Configure any lint rule

Every code listed by `strider lint --all-rules --list-rules` accepts the same
three options:

| Setting | Type | Default | Effect |
| --- | --- | --- | --- |
| `enabled` | boolean | Rule profile default | Include or omit the rule during an ordinary configured run. |
| `severity` | string | Rule default | Report as `"note"`, `"warning"`, or `"error"`. Any reported severity still makes the command exit `1`. |
| `excludes` | string list | `[]` | Skip only this rule on matching paths. |

```toml
[linter.rules.line-length-limit]
enabled = true
severity = "warning"
excludes = ["testdata/golden/**"]

[linter.rules.no-package-var]
enabled = false
```

Changing the severity of an extended rule does not implicitly enable it. Set
`enabled = true` as well, or select it on the CLI.

Rule-specific behavioral thresholds remain part of each rule's current
contract. Version 1 configures whether every rule runs, how it is classified,
and where it runs; it does not yet expose individual thresholds such as the
cyclomatic-complexity limit.

## Analyzer

Analyzer settings mirror the linter:

| Setting | Type | Default | Effect |
| --- | --- | --- | --- |
| `excludes` | string list | `[]` | Suppress analyzer results from matching files. Packages are still loaded so other files can be type-checked correctly. |
| `baseline` | string | unset | Apply this analysis baseline by default. |
| `baseline-variant` | string | `"loose"` | Variant used for newly generated analysis baselines. |
| `rules` | table | `{}` | Common configuration keyed by any registered analyzer code. |

All implemented analyzers are enabled by default. Every code listed by
`strider analyze --list-rules` accepts `enabled`, `severity`, and `excludes`
with the same types and behavior as lint rules.

```toml
[analyzer]
excludes = ["examples/broken/**"]

[analyzer.rules.deprecated-api-usage]
enabled = false

[analyzer.rules.possible-nil-dereference]
severity = "error"
excludes = ["internal/legacy/**"]
```

Analyzer codes are case-insensitive on `--only`, but configuration keys should
use the canonical kebab-case spelling printed by `--list-rules`.

## Baseline paths

The linter and analyzer have separate baselines because their diagnostic sets
and adoption timelines differ:

```toml
[linter]
baseline = "lint-baseline.toml"
baseline-variant = "loose"

[analyzer]
baseline = "analysis-baseline.toml"
baseline-variant = "strict"
```

Relative baseline paths resolve from the configuration directory, not the
shell's current directory. See the complete [baseline guide](/baselines/) for
generation, matching, pruning, backups, and CI adoption.

## Source directives

Configuration controls project policy; source directives record local
exceptions.

Skip formatting for an entire file:

```go
//strider:format-ignore
```

Suppress lint findings for the next declaration or statement:

```go
//strider:ignore no-package-var,no-init
```

Suppress lint findings for a whole file by placing the directive before the
package clause:

```go
//strider:ignore-file no-package-var
package example
```

Use the special code `all` to suppress every lint rule at that location.
Analyzer findings currently use configuration exclusions or baselines rather
than source suppression directives.
