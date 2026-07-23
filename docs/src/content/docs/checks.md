---
title: Checks
description: Run Strider's unified formatting, maintainability, and correctness checks.
---

`strider check` is the single diagnostic command. It combines formatting drift,
source-level policy, and package-aware correctness findings in one globally
sorted report. It is read-only unless `--fix` or `--fix-unsafe` is requested.
Files and parsed source are shared across selected checks, while more expensive
program information is prepared only when a selected check needs it.

Syntax trees, type information, and control-flow representations are internal
scheduling capabilities. They do not create separate commands, configuration
sections, or rule namespaces, and a check can change implementation without
changing its public code.

## Run checks

Run every check at or above the configured severity floor. By default this runs
warning and error checks:

```sh
strider check [PATH]...
```

Run checks whose effective severity is note or higher:

```sh
strider check --minimum-severity note [PATH]...
```

Run exactly a subset of codes:

```sh
strider check --only format --only no-init --only invalid-regexp [PATH]...
```

Comma-separated codes are equivalent:

```sh
strider check --only format,no-init,invalid-regexp [PATH]...
```

Explicit selection is case-insensitive and retains configured severities, the
minimum-severity threshold, and path exclusions. It also lets Strider avoid
preparing capabilities the selected checks do not need.

Run only checks whose effective severity is warning or higher:

```sh
strider check --minimum-severity warning ./...
```

Selection happens first, then per-rule severity overrides, then the minimum
threshold (`none < note < warning < error`). `--only` does not bypass the
threshold. Checks filtered this way are omitted before Strider plans CST, type,
or SSA work.

Keep an incremental text-mode session open while editing:

```sh
strider check --watch ./...
```

Watch mode reports the initial generation, then emits a fresh complete report
when selected source or the resulting findings change. It reuses unchanged
CST results but deliberately runs package-aware checks fresh instead of doing
extra package loads to prove a cached analysis reusable. Baseline generation
and pruning, JSON, and HTML are one-shot operations and cannot be combined
with `--watch`.

## Apply automatic fixes

Apply only automatic fixes explicitly classified as safe:

```sh
strider check --fix [PATH]...
```

Include safe, potentially unsafe, and unsafe automatic fixes:

```sh
strider check --fix-unsafe [PATH]...
```

The short forms are `-x` and `-u`. The two modes are mutually exclusive and
cannot be combined with watch mode, baseline generation, or baseline pruning.
The initial automatic set is `format`, `double-negation`,
`redundant-switch-break`, and `single-argument-append`.

Text reports put a `*` directly after checks with an automatic fix. Green marks
safe fixes and purple marks fixes that require `--fix-unsafe`; the same markers
appear in the per-check list, and the final issue summary counts both kinds.

Fix selection happens only after `--only`, effective severity, path exclusions,
source suppressions, and the active baseline have removed ineligible findings.
Baselined or suppressed findings are therefore not changed. Strider chooses an
explicitly automatic fix at the requested safety level, composes edits per
file, and skips every nonidentical overlapping fix with a warning. It then
formats affected source unless formatter exclusions or the file's format-ignore
directive opt it out, and validates that the result parses. A batch containing
safe changes also type-checks against an in-memory overlay before anything is
written.

The analyzed source is retained as a content snapshot. Every analyzed file is
compared after staging and before the first replacement, so a change detected
then aborts the whole batch as stale. Each write target is checked again
immediately before its replacement. After a successful application, Strider
runs the selected checks once more and reports what remains. Exit `0` means the
rerun is clean, exit `1` means findings remain, and exit `2` means selection,
validation, stale-source detection, or writing failed.

All outputs are staged before the first replacement, and each file replacement
is atomic. The batch is not a filesystem transaction: if a later rename fails
or a concurrent edit is detected after commit starts, an earlier replacement
can remain committed. Permission bits are preserved; ownership, ACLs, and
extended attributes depend on the host filesystem. A directly named source
symlink is kept and guarded against retargeting while its captured target is
updated.

Safe means the transformation is designed to preserve Go program semantics.
Parsing and type-checking provide strong guards against invalid output, but
they cannot prove identical observable behavior under every toolchain, build
tag, platform, or environment. Review unsafe fixes according to the code and
deployment contexts that matter to the project.

## Discover checks

Inspect the checks admitted by the effective severity floor:

```sh
strider check --list-checks
strider check --minimum-severity none --list-checks
strider check --explain invalid-regexp
```

The generated catalog statistics report the current inventory. Individual
check pages are grouped by purpose in the documentation sidebar.

## Configure checks

Every check code accepts `severity` and path `excludes` directly under the
version-1 `[checks.<code>]` namespace:

```toml
version = 1

[check]
minimum-severity = "warning"

[checks.format]
severity = "note"

[checks.file-length-limit]
severity = "error"
max-lines = 800
excludes = ["testdata/golden/**"]

[checks.unclosed-http-response-body]
severity = "error"

[checks.no-init]
severity = "none"
```

The tool-wide minimum severity, exclusions, and default baseline live under
`[check]`. Formatter layout and formatter-only exclusions remain under
`[formatter]`. See
[Configuration](/configuration/#checks) for the complete contract.

## Formatting findings

Formatting participates as code `format`. An unformatted file produces a normal
note at the start of the file, with a green `*` marking its safe automatic fix:

```text
format*: file is not formatted
  ┌─ main.go:1:1
  │
1 │ package main
2 │
```

Without a fix flag, `check` never writes the formatted candidate. Use
`strider fmt`, or `strider fmt --diff` to inspect the change. `check --fix`
applies the same validated candidate. Formatting findings are not captured by
baselines.

## Reports

Text is the default human-readable report format:

<div class="land-terminal report-terminal">
<pre><code><span class="t-note">no-init</span>: <strong>replace init with explicit initialization</strong>
   <span class="t-path">┌─ main.go:12:6</span>
   <span class="t-path">│</span>
11 <span class="t-path">│</span>
12 <span class="t-path">│</span> func <span class="t-note">init</span>() {
13 <span class="t-path">│</span>     configure()
<span aria-hidden="true"> </span>
<span class="t-note">no-init</span>  <span class="t-note">1</span>
<strong>1 issue:</strong> <span class="t-note">1 note</span></code></pre>
</div>

Use JSON for integrations or HTML for a self-contained artifact:

```sh
strider check --format json ./...
strider check --format html ./... > check-report.html
```

Each JSON diagnostic includes `code`, `message`, `severity`, `file`, `start`,
and `end`; diagnostics may also carry notes and suggested remedies. HTML reports
include severity totals, search, filters, and source context without external
assets or timestamps.
