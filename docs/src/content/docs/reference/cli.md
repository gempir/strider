---
title: CLI reference
description: Check and format commands, global options, reports, baselines, streams, and exit codes.
---

## Synopsis

```sh
strider [GLOBAL OPTIONS] COMMAND [OPTIONS] [FILE|DIR]...
```

| Command | Description |
| --- | --- |
| `strider help` | Print top-level usage. `-h` and `--help` are aliases. |
| `strider version` | Print the current version string. `-v` and `--version` are aliases. |
| `strider check` | Run formatting, maintainability, and correctness checks; optionally apply automatic fixes. |
| `strider fmt` | Format Go source in place. `format` is an alias. |

Calling Strider without a command is an error. Source commands recursively use
the current directory when no path is provided.

## Global options

Global options must precede the command.

Long options always use two dashes. Options with aliases also accept their
one-character form with one dash; aliases are scoped to their command.

| Flag | Description |
| --- | --- |
| `-c, --config PATH` | Use this `strider.toml` instead of automatic discovery. |
| `--config=PATH` | Equivalent inline form. |
| `-n, --no-config` | Disable discovery and use built-in defaults. |
| `-C, --color auto\|always\|never` | Control ANSI color. Default: configured value or `auto`. |
| `-C, --colors auto\|always\|never` | Alias for `--color`. |
| `--cache-dir PATH` | Override the persistent file-local result cache directory. |
| `--no-cache` | Disable persistent result cache reads and writes. |
| `--clear-cache` | Atomically clear versioned entries before running the command. |

`--config` and `--no-config` are mutually exclusive. Normally Strider searches
from the current directory through its parents and uses the nearest
`strider.toml`.

`auto` emits color only when the destination stream is a terminal. A non-empty
`NO_COLOR` disables color and a non-empty `FORCE_COLOR` forces it;
`FORCE_COLOR=0` explicitly disables it. `FORCE_COLOR` has highest precedence.
JSON and formatted source are never decorated with ANSI escapes.

Read-only `fmt --check` and `check` runs cache formatting status and native
syntax findings by exact source content, executable identity, effective
configuration, logical path, module identity, and target inputs. Absolute
paths, display positions, and effective severities are materialized for each
invocation rather than persisted. Candidate-producing diff, write, and fix
paths bypass the cache, as does watch mode. Corrupt or older-schema entries are
safe misses; cache writes are atomic and byte-bounded.

## `strider check`

```sh
strider check [OPTIONS] [FILE|DIR]...
```

`check` is read-only unless `--fix` or `--fix-unsafe` is requested. The default
warning floor runs warning and error checks. `--minimum-severity note` also includes notes, and
`--minimum-severity none` includes checks configured as `none`.

### Selection and reporting

| Flag | Description |
| --- | --- |
| `-f, --format text\|json\|html` | Select text, JSON, or a self-contained HTML report. Default: `text`. |
| `-o, --only CODE` | Run exactly these check codes. Repeatable, comma-separated, and case-insensitive. |
| `-s, --minimum-severity none\|note\|warning\|error` | Run only checks at or above this effective severity; overrides configuration. |
| `-q, --summary-only` | Print only per-check counts and the final aggregate issue summary. Text reports only. |
| `-l, --list-checks` | List the effective selected registry and severity, then exit. |
| `-e, --explain CODE` | Explain one selected check and show its effective severity, then exit. |
| `-w, --watch` | Keep a text-mode session open and rerun checks for each polled generation. |
| `-x, --fix` | Apply explicitly safe automatic fixes, then rerun the checks once. |
| `-u, --fix-unsafe` | Apply all automatic fixes, including potentially unsafe and unsafe fixes, then rerun once. |
| `--no-package-loading` | Skip package-aware checks that require Go package loading. Formatting and syntax checks still run. |

An unknown code supplied by configuration, `--only`, or `--explain` is an
exit-code `2` error. Explicit CLI selection preserves configured severity, the
minimum threshold, and path exclusions. Severity overrides are resolved before
the threshold, and `--only` does not bypass it.

Use `--only format` to check formatting without writing. Add `--fix` to apply
the validated result, use `strider fmt` for the focused write workflow, or use
`strider fmt --diff` to inspect it.

Use `--no-package-loading` when package metadata or dependencies are unavailable:

```sh
strider check --no-package-loading ./...
```

The same policy can be enabled persistently with `[check].package-loading = false`.
Package loading is required by fix mode because safe fixes are type-validated.

Watch mode prints a numbered full report for the initial generation and when
selected source or the resulting findings change. Package-aware checks run
fresh; unchanged CST work is reused. Watch requires text output and cannot be
combined with fix mode, baseline generation, or pruning.

### Automatic fixes

`--fix` and `--fix-unsafe` are mutually exclusive. The safe mode considers only
fixes explicitly marked both automatic and safe. Unsafe mode also considers
automatic fixes marked potentially unsafe or unsafe; it does not exclude safe
fixes. The initial automatic set covers `format`, `double-negation`,
`redundant-switch-break`, and `single-argument-append`.

Only diagnostics that survive check selection, effective-severity filtering,
path exclusions, source suppression, and baseline matching are considered.
Edits are composed in memory per file, nonidentical overlaps are skipped, and
affected source is formatted before validation unless formatter exclusions or
a format-ignore directive apply. Every result must parse. A batch containing
safe changes must also type-check through an overlay of the analyzed source.

The write is guarded by the analyzed content. Every source is compared after
staging and before commit, and each target is checked again immediately before
replacement. A stale comparison exits `2` and stops the remaining writes. After
a successful application, the selected checks run once more; the final report
and exit code describe the remaining findings.

Outputs are all staged before replacement and each replacement is atomic, but
the batch is not a cross-file transaction: a later rename failure or concurrent
edit detected after commit starts can leave an earlier file updated. Permission
bits are preserved. Ownership, ACLs, and extended attributes are
filesystem-dependent.

Safe fixes are designed to preserve Go program semantics. A successful parse
and type-check is not a proof of identical behavior across all toolchains,
build tags, platforms, and environments.

### Baselines

| Flag | Description |
| --- | --- |
| `-b, --baseline PATH` | Apply or update this baseline; overrides `[check].baseline`. |
| `-g, --generate-baseline` | Replace the selected baseline with all current non-format findings and exit `0`. |
| `-r, --remove-outdated-baseline-entries` | Remove baseline entries that no longer match; never add new findings. |

Generation and pruning require either `--baseline PATH` or
`[check].baseline`. They are mutually exclusive. Code `format` is never stored
in a baseline. Fix mode can use an ordinary baseline, but cannot be combined
with generation or pruning.

## `strider fmt`

```sh
strider fmt [--check|--diff|--write|--stdin] [FILE|DIR]...
```

| Flag | Description |
| --- | --- |
| `-c, --check` | Report files that would change without writing. |
| `-d, --diff` | Print full unified diffs without writing. |
| `-w, --write` | Write files in place. Writing is already the default filesystem mode. |
| `-s, --stdin` | Read one source file from standard input and write it to standard output. |
| `-f, --stdin-filename PATH` | Set the logical standard-input filename; default `<stdin>`. |

`--stdin-filename` requires `--stdin`, and standard-input mode cannot be
combined with paths or filesystem mode flags. Formatter width, indentation,
line endings, and exclusions come from `[formatter]` in `strider.toml`.

## Paths and discovery

Paths may name Go files, directories, or recursive `./...` notation. With no
path, Strider uses `.` recursively. Discovery includes test files and is
deterministic.

The `.git`, `.hg`, `.svn`, and `vendor` directories are skipped. Directory
walks skip symlink entries, while a directly named source symlink is followed.
Fix mode preserves that link and updates its captured target while guarding
against retargeting. Files carrying the standard
`// Code generated ... DO NOT EDIT.` marker are skipped. Configuration can add
tool-wide and per-check exclusions.

## Streams

- Formatted source, diffs, check lists, explanations, and visible diagnostics go
  to standard output.
- Usage errors, parsing or package-loading failures, unsupported syntax,
  baseline failures, stale-baseline warnings, and skipped-fix warnings go to
  standard error.
- Successful baseline generation writes the file without printing diagnostics.

Text diagnostics use a rich, source-annotated layout with a severity-colored
heading and source span, file location, one surrounding line on either side,
notes or suggested remedies, and an aggregate severity summary. Redirected
output remains plain under the default `auto` mode.

Severity-bearing rule codes use the same color as their severity: red for
errors, yellow for warnings, and blue for notes.

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | Clean after any requested fixes, successfully formatted, or baseline generation completed. |
| `1` | One or more visible check findings remain, including formatting drift. |
| `2` | Invalid command/options, configuration or baseline error, fix validation or stale-source failure, parse or package-load failure, unsupported syntax, or I/O failure. |

Any visible diagnostic currently causes exit `1`, regardless of whether its
configured severity is `note`, `warning`, or `error`.
