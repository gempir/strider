---
title: CLI reference
description: Check and format commands, global options, reports, baselines, streams, and exit codes.
---

## Synopsis

```text
strider [--config PATH|--no-config] [--color auto|always|never] COMMAND [OPTIONS] [FILE|DIR]...
```

| Command | Description |
| --- | --- |
| `strider help` | Print top-level usage. `-h` and `--help` are aliases. |
| `strider version` | Print the current version string. `--version` is an alias. |
| `strider check` | Run formatting, maintainability, and correctness checks without writing source. |
| `strider fmt` | Format Go source in place. `format` is an alias. |

Calling Strider without a command is an error. Source commands recursively use
the current directory when no path is provided.

## Global options

Global options must precede the command.

| Flag | Description |
| --- | --- |
| `--config PATH` | Use this `strider.toml` instead of automatic discovery. |
| `--config=PATH` | Equivalent inline form. |
| `--no-config` | Disable discovery and use built-in defaults. |
| `--color auto\|always\|never` | Control ANSI color. Default: configured value or `auto`. |
| `--colors auto\|always\|never` | Alias for `--color`. |

`--config` and `--no-config` are mutually exclusive. Normally Strider searches
from the current directory through its parents and uses the nearest
`strider.toml`.

`auto` emits color only when the destination stream is a terminal. A non-empty
`NO_COLOR` disables color and a non-empty `FORCE_COLOR` forces it;
`FORCE_COLOR=0` explicitly disables it. `FORCE_COLOR` has highest precedence.
JSON and formatted source are never decorated with ANSI escapes.

## `strider check`

```text
strider check [OPTIONS] [FILE|DIR]...
```

`check` is read-only. Its default profile contains 94 checks, including code
`format`; `--all` enables the complete 203-check catalog.

### Selection and reporting

| Flag | Description |
| --- | --- |
| `--format text\|json\|html` | Select text, JSON, or a self-contained HTML report. Default: `text`. |
| `--only CODE` | Run exactly these check codes. Repeatable, comma-separated, and case-insensitive. |
| `--all` | Enable every built-in check. Mutually exclusive with `--only`. |
| `--list-checks` | List the effective selected registry and severity, then exit. |
| `--explain CODE` | Explain one selected check and show its effective severity, then exit. |
| `--watch` | Keep a text-mode incremental session open and rerun changed generations. |

An unknown code supplied by configuration, `--only`, or `--explain` is an
exit-code `2` error. Explicit CLI selection overrides configured `enabled`
states but preserves configured severity and path exclusions.

Use `--only format` to check formatting without writing. Use `strider fmt` to
write the suggested result or `strider fmt --diff` to inspect it.

Watch mode prints a numbered full report for the initial generation and each
detected source or package-boundary change. It requires text output and cannot
be combined with baseline generation, pruning, or backup.

### Baselines

| Flag | Description |
| --- | --- |
| `--baseline PATH` | Apply or update this baseline; overrides `[checks].baseline`. |
| `--baseline-variant loose\|strict` | Choose the shape used by the next generation; overrides configuration. |
| `--generate-baseline` | Replace the selected baseline with all current non-format findings and exit `0`. |
| `--ignore-baseline` | Run without applying the configured or explicit baseline. |
| `--remove-outdated-baseline-entries` | Remove baseline entries that no longer match; never add new findings. |
| `--backup-baseline` | Before generation or pruning, copy an existing file to `<path>.bkp`. |

Generation and pruning require either `--baseline PATH` or
`[checks].baseline`. They are mutually exclusive. `--backup-baseline` requires
one of those update operations, and `--ignore-baseline` cannot be combined with
an update. Code `format` is never stored in a baseline.

## `strider fmt`

```text
strider fmt [--diff|--write|--stdin] [FILE|DIR]...
```

| Flag | Description |
| --- | --- |
| `--diff` | Print full unified diffs without writing. |
| `--write` | Write files in place. Writing is already the default filesystem mode. |
| `--stdin` | Read one source file from standard input and write it to standard output. |
| `--stdin-filename PATH` | Set the logical standard-input filename; default `<stdin>`. |

`--stdin-filename` requires `--stdin`, and standard-input mode cannot be
combined with paths or filesystem mode flags. Formatter width, indentation,
line endings, and exclusions come from `[formatter]` in `strider.toml`.

## Paths and discovery

Paths may name Go files, directories, or recursive `./...` notation. With no
path, Strider uses `.` recursively. Discovery includes test files and is
deterministic.

The `.git`, `.hg`, `.svn`, and `vendor` directories are skipped. Symlinked Go
files and files carrying the standard `// Code generated ... DO NOT EDIT.`
marker are skipped. Configuration can add tool-wide and per-check exclusions.

## Streams

- Formatted source, diffs, check lists, explanations, and visible diagnostics go
  to standard output.
- Usage errors, parsing or package-loading failures, unsupported syntax,
  baseline failures, and stale-baseline warnings go to standard error.
- Successful baseline generation writes the file without printing diagnostics.

Text diagnostics use a rich, source-annotated layout with a severity-colored
heading, file location, source line, underlined span, notes or suggested
remedies, and an aggregate severity summary. Redirected output remains plain
under the default `auto` mode.

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | Clean, successfully formatted, or baseline generation completed. |
| `1` | One or more visible check findings, including formatting drift. |
| `2` | Invalid command/options, configuration or baseline error, parse or package-load failure, unsupported syntax, or I/O failure. |

Any visible diagnostic currently causes exit `1`, regardless of whether its
configured severity is `note`, `warning`, or `error`.
