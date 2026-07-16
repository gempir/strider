---
title: CLI reference
description: Commands, global configuration, baseline flags, streams, and exit codes.
---

## Synopsis

```text
strider [--config PATH|--no-config] [--color auto|always|never] COMMAND [OPTIONS] [FILE|DIR]...
```

| Command | Description |
| --- | --- |
| `strider help` | Print top-level usage. `-h` and `--help` are aliases. |
| `strider version` | Print the current version string. `--version` is an alias. |
| `strider fmt` | Format Go source. `format` is an alias. |
| `strider lint` | Run the file-local CST lint rules. |
| `strider analyze` | Run package-aware static-analysis checks. |

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
| `--colors auto\|always\|never` | Alias for `--color`, matching Mago's spelling. |

`--config` and `--no-config` are mutually exclusive. Normally Strider searches
from the current directory through its parents and uses the nearest
`strider.toml`.

`auto` emits color only when the destination stream is a terminal. A non-empty
`NO_COLOR` disables color and a non-empty `FORCE_COLOR` forces it;
`FORCE_COLOR=0` explicitly disables it. `FORCE_COLOR` has highest precedence.
JSON and formatted source are never decorated with ANSI escapes.

## `strider fmt`

```text
strider fmt [--check|--diff|--write|--stdin] [FILE|DIR]...
```

| Flag | Description |
| --- | --- |
| `--check` | Print styled `would reformat` notices and exit `1`; do not write. |
| `--diff` | Print full unified diffs and exit `1`; do not write. |
| `--write` | Write files in place. Writing is already the default filesystem mode. |
| `--stdin` | Read one source file from standard input and write it to standard output. |
| `--stdin-filename PATH` | Set the logical standard-input filename; default `<stdin>`. |

Mode flags are mutually exclusive. `--stdin-filename` requires `--stdin`, and
standard-input mode cannot be combined with paths or filesystem mode flags.

Formatter width, indentation width, line endings, and filesystem exclusions
come from `[formatter]` in `strider.toml`.

## `strider lint`

```text
strider lint [OPTIONS] [FILE|DIR]...
```

### Selection and reporting

| Flag | Description |
| --- | --- |
| `--format text\|json` | Select text or JSON diagnostics. Default: `text`. |
| `--only CODE` | Select rule codes. Repeatable and comma-separated. |
| `--all-rules` | Enable all 116 built-in rules. Mutually exclusive with `--only`. |
| `--list-rules` | List the effective selected registry and severity, then exit. |
| `--explain CODE` | Explain one selected rule and show its effective severity, then exit. |

An unknown code passed through configuration, `--only`, or `--explain` is an
exit-code `2` error. Explicit CLI selection overrides configured `enabled`
states but preserves configured severity and path exclusions.

### Baselines

| Flag | Description |
| --- | --- |
| `--baseline PATH` | Apply or update this lint baseline; overrides `[linter].baseline`. |
| `--baseline-variant loose\|strict` | Choose the shape used by the next generation; overrides configuration. |
| `--generate-baseline` | Replace the selected baseline with all current findings and exit `0`. |
| `--ignore-baseline` | Run without applying the configured or explicit baseline. |
| `--remove-outdated-baseline-entries` | Remove baseline entries that no longer match; never add new findings. |
| `--backup-baseline` | Before generation or pruning, copy an existing file to `<path>.bkp`. |

Generation and pruning require either `--baseline PATH` or `[linter].baseline`.
They are mutually exclusive. `--backup-baseline` requires one of those update
operations, and `--ignore-baseline` cannot be combined with an update.

## `strider analyze`

```text
strider analyze [OPTIONS] [FILE|DIR]...
```

### Selection and reporting

| Flag | Description |
| --- | --- |
| `--format text\|json` | Select text or JSON diagnostics. Default: `text`. |
| `--only CODE` | Select analyzer codes. Repeatable, comma-separated, and case-insensitive. |
| `--list-rules` | List the effective selected registry and severity, then exit. |
| `--explain CODE` | Explain one selected analyzer and show its effective severity, then exit. |

The analyzer loads and type-checks complete packages and constructs SSA.
Analyzer codes use canonical kebab-case names such as `invalid-regexp`.

### Baselines

`analyze` supports the same six baseline flags as `lint`:
`--baseline`, `--baseline-variant`, `--generate-baseline`,
`--ignore-baseline`, `--remove-outdated-baseline-entries`, and
`--backup-baseline`. Its configured defaults come from `[analyzer]`, and its
baseline should be separate from the linter's.

## Paths and discovery

Paths may name Go files, directories, or recursive `./...` notation. With no
path, Strider uses `.` recursively. Discovery includes test files and is
deterministic.

The `.git`, `.hg`, `.svn`, and `vendor` directories are skipped. Symlinked Go
files and files carrying the standard `// Code generated ... DO NOT EDIT.`
marker are skipped. Configuration can add tool-wide and per-rule exclusions.

## Streams

- Formatted source, diffs, changed paths, rule lists, explanations, and visible
  lint/analyze diagnostics go to standard output.
- Usage errors, parsing failures, unsupported syntax, baseline failures, and
  stale-baseline warnings go to standard error.
- Successful baseline generation writes the file without printing diagnostics.

Text diagnostics use a rich, source-annotated layout with a severity-colored
heading, file location, source line, underlined span, notes/fixes, and an
aggregate severity summary. Redirected output remains plain under the default
`auto` mode.

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | Clean, successfully formatted, or baseline generation completed. |
| `1` | Visible lint or analysis findings, or formatting differences. |
| `2` | Invalid command/options, configuration or baseline error, parse failure, unsupported syntax, or I/O failure. |

Any visible diagnostic currently causes exit `1`, regardless of whether its
configured severity is `note`, `warning`, or `error`.
