<img src="docs/src/assets/strider.png" alt="Strider" width="200">

# Strider

Strider is an experimental Go 1.26 formatter and code checker. One read-only
`strider check` run reports formatting drift, style and maintainability issues,
and package-aware correctness problems in one deterministic diagnostic stream.
`strider fmt` remains the focused command for writing formatted source.

# Slopclaimer

This is slop, written heavily with LLMs. I don't have the time next to a full
time job to build this level of product without LLMs. The good thing though,
none of this code ever runs in production. You run it in CI or locally and get
useful output or not.

## Inspiration

Strider is hugely inspired by
[carthage-software/mago](https://github.com/carthage-software/mago), particularly
its speed, configuration design, and reporting.

## Check

Run all checks enabled by the project configuration:

```sh
strider check ./...
strider check --format json ./...
strider check --format html ./... > check-report.html
strider check --only format,no-init,invalid-regexp ./...
strider check --minimum-severity warning ./...
strider check --watch ./...
strider check --list-checks
strider check --explain invalid-regexp
```

Long options always require two dashes. Every option also has a scoped
one-character alias, such as `-s warning` for `--minimum-severity warning` and
`-w` for `--watch`.

The built-in profile selects 118 checks. With the default warning severity
floor, 96 warning and error checks run; use `--minimum-severity note` to include
the profile's notes. `strider check --all --minimum-severity note` enables the
complete 227-check catalog. `--only` selects exactly the named codes and avoids
building program information that those checks do not need.

Formatting is the `format` check. It reports an unformatted file without
modifying it and suggests `strider fmt` as the remedy. Strider internally
schedules only the source representations required by the selected checks;
those implementation capabilities are not separate user-facing tools.

`--watch` keeps a text-mode check session alive. Unchanged concrete syntax and
proven-equivalent package results are reused between generations; edits emit a
fresh complete report without modifying source.

## Format

```sh
strider fmt ./...
strider fmt --diff ./...
strider fmt --stdin --stdin-filename file.go < file.go
```

With file or directory arguments, `fmt` writes in place. With no path, it
recursively formats the current directory. `--diff` is read-only, while
`--stdin` reads one file and writes formatted source to standard output. A file
containing `//strider:format-ignore` is passed through unchanged.

The formatter supports ordinary application code, including generics, type
switches, `select`, channel sends, `goto`, `fallthrough`, and labeled
statements. Some comments embedded deeply inside expressions remain outside the
current syntax boundary.

## Configuration

Strider discovers the nearest `strider.toml` from the current directory upward.
Version 1 uses one `[checks]` namespace. Every check supports `enabled`,
`severity`, and path `excludes`; the formatter exposes print width, visual
indentation width, line endings, and filesystem exclusions.

```toml
version = 1
color = "auto"

[formatter]
print-width = 180
max-empty-lines = 1

[checks]
baseline = "strider-baseline.toml"
minimum-severity = "warning"

[checks.rules.line-length-limit]
enabled = true
severity = "warning"

[checks.rules.possible-nil-dereference]
severity = "error"
excludes = ["internal/legacy/**"]
```

Checks resolve their built-in or configured severity before applying
`minimum-severity`. The order is `note < warning < error`, so a rule promoted
to `error` still runs in an error-only profile and one demoted to `note` does
not. `--minimum-severity` overrides the configured threshold for one run.

Use `strider --config PATH COMMAND` to select a file explicitly or
`strider --no-config COMMAND` to run with built-in defaults. Rich terminal
output uses color automatically; set `color = "always"` or `"never"`, or
override it with `strider --color always|never COMMAND`. `NO_COLOR` and
`FORCE_COLOR` are also honored. The schema is strict: unknown keys and check
codes are errors.

## Baselines

A single baseline records existing check findings without hiding new ones:

```sh
strider check --generate-baseline --baseline strider-baseline.toml ./...
```

Configure the path for ordinary runs:

```toml
[checks]
baseline = "strider-baseline.toml"
baseline-variant = "loose"
```

Formatting findings are intentionally never captured in a baseline. Loose
baselines match file, code, message, and count while surviving line movement;
strict baselines match exact line ranges. Use `--ignore-baseline` to see the
full backlog and `--remove-outdated-baseline-entries` to prune fixed issues
without absorbing new findings.

## Exit codes

- `0`: success with no visible findings, or a baseline update completed.
- `1`: one or more visible check findings, including formatting drift.
- `2`: command, configuration, baseline, parsing, package-loading,
  unsupported-syntax, or I/O error.

## Open-source corpus

`make corpus-check` runs formatting and the check catalog against pinned popular
Go projects. It rejects processing errors, compares deterministic output with a
reviewed baseline, and enforces per-project timing budgets. CI publishes the
timing table in its job summary and uploads standalone JSON and HTML reports.
