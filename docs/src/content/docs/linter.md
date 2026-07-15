---
title: Linter
description: Run Strider's fast AST-only lint engine and manage findings.
---

Strider's linter parses each file once and dispatches AST nodes only to rules
that registered interest in that node type. Files run concurrently up to
`GOMAXPROCS`; diagnostics are sorted by path, byte offset, and rule code before
reporting, so output remains deterministic.

## Run rules

Run every rule:

```sh
strider lint [PATH]...
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

See the [lint-rule reference](/rules/) for the complete behavior and examples
for every rule.

## Reports

Text is the default human-readable report format:

```text
main.go:12:6: warning[no-init]: replace init with explicit initialization
```

Use JSON for integrations:

```sh
strider lint --format json
```

Each JSON diagnostic includes `code`, `message`, `severity`, `file`, `start`,
and `end`. The shared model can also carry `notes` and `fixes`, although the
initial rules do not currently apply fixes.

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
