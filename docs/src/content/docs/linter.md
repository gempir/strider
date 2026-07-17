---
title: Linter
description: Run Strider's fast lossless-CST lint engine and manage findings.
---

Strider's linter parses each file once into a lossless concrete syntax tree
(CST), then dispatches concrete grammar nodes to the registered rules. Comments,
whitespace, separators, and original token spelling remain available to checks.
Files run concurrently up to
`GOMAXPROCS`; diagnostics are sorted by path, byte offset, and rule code before
reporting, so output remains deterministic.

Lint rules stay deliberately file-local: they cover style, naming,
readability, complexity, and syntactic maintainability without loading or
type-checking a package. Checks that need resolved APIs, package metadata,
constants propagated through values, or control-flow analysis belong to
[`strider analyze`](/analyzers/).

## Run rules

Run the configured rule set (the seven-rule profile by default):

```sh
strider lint [PATH]...
```

Run the complete built-in registry:

```sh
strider lint --all-rules [PATH]...
```

Run a subset:

```sh
strider lint --only no-init --only no-package-var [PATH]...
```

Comma-separated codes are equivalent:

```sh
strider lint --only no-init,no-package-var [PATH]...
```

## Discover rules

The registry is the source for the built-in command help:

```sh
strider lint --list-rules
strider lint --explain cyclomatic-complexity
```

See the [lint reference](/lints/) for the complete behavior and examples
for every rule.

## Configure rules

Every registered lint code accepts `enabled`, `severity`, and path `excludes`
in `strider.toml`. The seven profile rules are enabled by default; extended
rules can be enabled individually.

```toml
[linter.rules.no-package-var]
enabled = false

[linter.rules.line-length-limit]
enabled = true
severity = "error"
excludes = ["testdata/golden/**"]
```

Tool-wide exclusions and a default baseline also live under `[linter]`. See
[Configuration](/configuration/#linter) for the complete contract.

## Adopt with a baseline

Record current debt while keeping new findings visible:

```sh
strider lint --generate-baseline --baseline lint-baseline.toml ./...
```

Configure that path for ordinary runs, temporarily bypass it with
`--ignore-baseline`, and safely remove fixed entries with
`--remove-outdated-baseline-entries`. The [baseline guide](/baselines/) covers
variants and the recommended lifecycle.

## Reports

Text is the default human-readable report format:

```text
warning[no-init]: replace init with explicit initialization
  ┌─ main.go:12:1
  │
12 │ func init() {
   │ ^^^^^^^^^^^^^

found 1 issue: 1 warning
```

On a terminal, severity, rule code, path, source gutter, and underline use
distinct semantic colors. Use the global `--color` option or top-level
configuration setting to override automatic terminal detection.

Use JSON for integrations:

```sh
strider lint --format json
```

Each JSON diagnostic includes `code`, `message`, `severity`, `file`, `start`,
and `end`. The shared model can also carry `notes` and `fixes`, although the
initial rules do not currently apply fixes.

Use HTML for a self-contained report that can be opened locally, uploaded as a
CI artifact, or published with project documentation:

```sh
strider lint --format html ./... > lint-report.html
```

The report includes severity totals, search and severity filters, and details
for every finding. It has no external assets and contains no timestamps, so the
same diagnostics produce the same page on every run.

## Suppressions

Suppress selected codes on the next syntactic declaration or statement:

```go
//strider:ignore no-package-var
var registry = newRegistry()
```

Suppress a whole file by placing the directive before `package`:

```go
//strider:ignore-file no-package-var,no-init
package legacy
```

Use `all` in place of a rule code to suppress all rules. A suppression on a
function or statement also applies to findings produced by its descendants.

Suppressions should document intentional exceptions. They are not checked
expectations: Strider does not yet report stale or unused suppression codes.
