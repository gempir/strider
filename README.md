<img src="docs/src/assets/strider.png" alt="Strider" width="200">

# Strider

Strider is an experimental Go 1.26 formatter and code checker. One
`strider check` run reports formatting drift, style and maintainability issues,
and package-aware correctness problems in one deterministic diagnostic stream.
It is read-only unless `--fix` or `--fix-unsafe` is requested. `strider fmt`
remains the focused command for writing formatted source.

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

Run all checks at or above the configured severity floor:

```sh
strider check ./...
strider check --format json ./...
strider check --format html ./... > check-report.html
strider check --only format,no-init,invalid-regexp ./...
strider check --minimum-severity warning ./...
strider check --fix ./...
strider check --fix-unsafe ./...
strider check --summary-only ./...
strider check --watch ./...
strider check --list-checks
strider check --explain invalid-regexp
```

Long options always require two dashes. Every option also has a scoped
one-character alias, such as `-s warning` for `--minimum-severity warning` and
`-w` for `--watch`.

The default warning severity floor runs warning and error checks; use
`--minimum-severity note` to include notes too.
Checks configured with `severity = "none"` are suppressed unless the command
uses `--minimum-severity none`. `--only` selects exactly the named codes and
avoids building program information that those checks do not need.

`-x, --fix` applies only automatic fixes explicitly classified as safe.
`-u, --fix-unsafe` includes safe, potentially unsafe, and unsafe automatic
fixes. The flags are mutually exclusive. The initial automatic set covers
`format`, `double-negation`, `redundant-switch-break`, and
`single-argument-append`. Fix mode cannot be combined with watch mode, baseline
generation, or baseline pruning.

Only findings left after selection, severity filtering, exclusions,
suppression, and baseline matching are eligible. Strider composes edits in
memory, skips nonidentical overlaps, formats affected source unless formatter
exclusions apply, verifies that it parses, and type-checks safe changes through
an overlay. Every analyzed source is compared with its snapshot after staging,
before commit, and each target is checked again immediately before replacement.
A detected concurrent edit stops the remaining writes. Checks then run once
more, so the report and exit status describe the remaining findings.

Safe fixes are designed to preserve Go program semantics, and type-checking
catches many invalid rewrites. Neither classification nor validation proves
identical behavior across every toolchain, build tag, platform, or environment.

Formatting is the `format` check. It reports an unformatted file without
modifying it by default and suggests `strider fmt` as the remedy. Fix mode can
apply the same validated formatter result. Strider internally schedules only
the source representations required by the selected checks; those
implementation capabilities are not separate user-facing tools.

`--watch` keeps a text-mode check session alive. Unchanged concrete syntax is
reused between generations while package-aware checks run fresh; selected
source changes or changed findings emit a complete report without modifying
source. Watch and fix modes cannot be combined.

## Format

```sh
strider fmt ./...
strider fmt --diff ./...
strider fmt --stdin --stdin-filename file.go < file.go
```

With file or directory arguments, `fmt` writes in place. With no path, it
recursively formats the current directory. `--diff` is read-only, while
`--stdin` reads one file and writes formatted source to standard output. A file
with `//strider:format-ignore` in its header before the package clause is passed
through unchanged.

The formatter supports ordinary application code, including generics, type
switches, `select`, channel sends, `goto`, `fallthrough`, and labeled
statements. Some comments embedded deeply inside expressions remain outside the
current syntax boundary.

## Configuration

Strider discovers the nearest `strider.toml` from the current directory upward.
Version 1 uses `[check]` for command-wide policy and `[checks.<code>]` for
individual checks. Every check supports `severity` and path `excludes`; the
formatter exposes only its selected width and filesystem exclusions.

```toml
version = 1
color = "auto"

[formatter]
print-width = 180
excludes = ["internal/generated/**"]

[check]
baseline = "strider-baseline.toml"
minimum-severity = "warning"

[checks.file-length-limit]
severity = "warning"
max-lines = 800

[checks.unclosed-http-response-body]
severity = "error"
excludes = ["internal/legacy/**"]
```

Checks resolve their built-in or configured severity before applying
`minimum-severity`. The order is `none < note < warning < error`, so a rule promoted
to `error` still runs in an error-only profile and one demoted to `note` does
not. A rule set to `none` is normally disabled, while `--minimum-severity none`
still provides an explicit way to run it. `--minimum-severity` overrides the
configured threshold for one run.

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
[check]
baseline = "strider-baseline.toml"
```

Formatting findings are intentionally never captured in a baseline. Baselines
strictly match exact file, code, and line ranges. Use
`--remove-outdated-baseline-entries` to prune fixed issues without absorbing
new findings.

## Exit codes

- `0`: success with no visible findings after any requested fixes, or a
  baseline update completed.
- `1`: one or more visible check findings remain, including formatting drift.
- `2`: command, configuration, baseline, fix validation, stale source,
  parsing, package-loading, unsupported-syntax, or I/O error.

## Open-source corpus

`make corpus-check` runs formatting and the check catalog against pinned popular
Go projects. It rejects processing errors, compares deterministic output with a
reviewed baseline, and enforces per-project timing budgets. CI publishes the
timing table in its job summary and uploads standalone JSON and HTML reports.
