---
title: cyclomatic-complexity
description: Limit branching complexity in declared functions.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

**Maximum:** `10`  
**Configuration:** `severity` and path `excludes`; maximum remains `10`

Reports a declared function when its calculated complexity is greater than
10. A low complexity score makes control flow easier to understand, test, and
change safely.

## How the score is calculated

Every function starts at `1`. Strider adds one for each:

- `if`, `for`, `range`, or type-switch statement;
- non-default `case` clause;
- non-default `select` communication clause; and
- `&&` or `||` binary expression.

The rule currently runs on function declarations. Branches in a function
literal nested inside a declaration contribute to the enclosing declaration's
score.

## Bad

```go
func route() string {
	if first() { return "first" }
	if second() { return "second" }
	if third() { return "third" }
	if fourth() { return "fourth" }
	if fifth() { return "fifth" }
	if sixth() { return "sixth" }
	if seventh() { return "seventh" }
	if eighth() { return "eighth" }
	if ninth() { return "ninth" }
	if tenth() { return "tenth" }
	return "fallback"
}
```

## Good

```go
func route(value any, state routeState) string {
	if text, ok := value.(string); ok {
		return routeText(text, state)
	}
	return routeOther(value, state)
}
```

Extract cohesive decisions into named helpers. Avoid splitting code solely to
satisfy the number; each helper should represent a meaningful unit.

## Suppress

```go
//strider:ignore cyclomatic-complexity
func generatedStateMachine() {
	// Intentionally mirrors an external state table.
}
```
