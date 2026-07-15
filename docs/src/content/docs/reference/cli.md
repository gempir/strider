---
title: CLI reference
description: Commands, arguments, output streams, and exit codes.
---

## Global commands

| Command | Description |
| --- | --- |
| `strider help` | Print top-level usage. `-h` and `--help` are aliases. |
| `strider version` | Print the current version string. `--version` is an alias. |
| `strider fmt` | Format Go source. `format` is an alias. |
| `strider lint` | Run the AST-only lint rules. |

Calling Strider without a command is an error. Source-oriented commands use
the current directory recursively when no path is provided.

## `strider fmt`

```text
strider fmt [--check|--diff|--write|--stdin] [FILE|DIR]...
```

| Flag | Description |
| --- | --- |
| `--check` | Print paths that differ and exit `1`; do not write. |
| `--diff` | Print full unified diffs and exit `1`; do not write. |
| `--write` | Write files in place. Writing is already the default filesystem mode. |
| `--stdin` | Read one source file from standard input and write it to standard output. |
| `--stdin-filename PATH` | Set the logical standard-input filename; default `<stdin>`. |

Mode flags are mutually exclusive. `--stdin-filename` requires `--stdin`, and
standard-input mode cannot be combined with paths or filesystem mode flags.

## `strider lint`

```text
strider lint [OPTIONS] [FILE|DIR]...
```

| Flag | Description |
| --- | --- |
| `--format text\|json` | Select text or JSON diagnostics. Default: `text`. |
| `--only CODE` | Select rule codes. Repeatable and comma-separated. |
| `--all-rules` | Enable all 111 built-in rules. Mutually exclusive with `--only`. |
| `--list-rules` | List the selected registry and exit. |
| `--explain CODE` | Explain one selected rule and exit. |

An unknown code passed to `--only` or `--explain` is an exit-code `2` error.
`--only` also limits what appears in `--list-rules` and what can be selected by
`--explain`.

## Streams

- Formatted source, diffs, changed paths, rule lists, explanations, and lint
  reports go to standard output.
- Usage errors, parsing failures, unsupported syntax, and I/O failures go to
  standard error.

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | Clean or successful. |
| `1` | Lint findings or formatting differences. |
| `2` | Invalid command/options, parse failure, unsupported syntax, or I/O failure. |
