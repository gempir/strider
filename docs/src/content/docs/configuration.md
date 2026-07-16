---
title: Configuration
description: Every setting and command option supported by the current Strider draft.
---

## Configuration status

Strider does **not** currently read a `strider.toml` file, environment
overrides, or global user configuration. Unknown configuration files therefore
have no effect. Current behavior is controlled through command-line options,
source directives, and a small set of fixed defaults.

The configuration contract remains provisional. A strict, versioned TOML
schema is planned only after the formatter and linter behavior stabilizes.

## Formatter options

| Option | Default | Effect |
| --- | --- | --- |
| `--check` | Off | Print files that would change. Never writes. |
| `--diff` | Off | Print full unified diffs. Never writes. |
| `--write` | Off explicitly | Write formatted files in place. This is the effective mode when no mode option is supplied. |
| `--stdin` | Off | Read one Go source file from standard input and write formatted source to standard output. |
| `--stdin-filename PATH` | `<stdin>` | Logical filename used while formatting standard input. Requires `--stdin`. |

`--check`, `--diff`, and `--write` are mutually exclusive. Standard-input mode
does not accept paths or any of those three mode options.

## Linter options

| Option | Default | Effect |
| --- | --- | --- |
| `--format text\|json` | `text` | Select the diagnostic report format. |
| `--only CODE` | Seven-rule profile | Run only the selected code. Repeat the option or provide comma-separated codes. |
| `--all-rules` | Off | Enable all 116 native rules. Mutually exclusive with `--only`. |
| `--list-rules` | Off | List selected rule codes, severities, and summaries, then exit. |
| `--explain CODE` | None | Print the registry explanation and examples for one selected rule, then exit. |

The seven Strider profile rules are enabled at `warning` severity by default.
The 109 additional rules are selectable individually or together with
`--all-rules`. Rule severities and thresholds cannot yet be changed; each
rule reference records the fixed default used when that rule is enabled.

## Analyzer options

| Option | Default | Effect |
| --- | --- | --- |
| `--format text\|json` | `text` | Select the diagnostic report format. |
| `--only CODE` | All implemented checks | Run only the selected analysis code. Repeatable, comma-separated, and case-insensitive. |
| `--list-rules` | Off | List selected analysis codes, severities, and summaries, then exit. |
| `--explain CODE` | None | Print the explanation and examples for one selected analysis rule, then exit. |

## Fixed formatter settings

| Setting | Current value |
| --- | --- |
| Print width | 100 columns |
| Indentation | Tabs; spaces are reserved for alignment |
| Line endings | LF |
| End of file | Exactly one final newline |
| Imports | Sorted into standard library, third-party, and current-module groups |
| Broken lists | One item per line with a mandatory trailing comma |
| Binary wrapping | Operators remain at the end of the preceding line |
| Generated files | Skipped |

These settings form one strict Strider profile and have no command-line
overrides.

## Source discovery

Paths may be Go files, directories, or recursive `./...` notation. When no
path is supplied, Strider uses `.` and walks recursively. Discovery is
deterministic and includes test files.

The `.git`, `.hg`, `.svn`, and `vendor` directories are skipped. Symlinked Go
files and files with the standard `// Code generated ... DO NOT EDIT.` marker
are also skipped.

## Source directives

Skip formatting for an entire file:

```go
//strider:format-ignore
```

Suppress linter findings for the next declaration or statement:

```go
//strider:ignore no-package-var,no-init
```

Suppress findings for a whole file by placing this directive before the
package clause:

```go
//strider:ignore-file no-package-var
package example
```

Use the special code `all` to suppress every rule at that location. Strider
does not yet support checked `expect` directives or region suppressions.
