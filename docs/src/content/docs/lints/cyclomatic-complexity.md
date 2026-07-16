---
title: cyclomatic-complexity
description: Limit branching complexity in declared functions.
---

**Default severity:** `warning`  
**Maximum:** `10`  
**Configuration:** `enabled`, `severity`, and path `excludes`; maximum remains `10`

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
func route(value any, ready, fallback bool) string {
	switch current := value.(type) {
	case string:
		if ready && !fallback {
			return current
		}
	case int:
		if current > 0 || fallback {
			return strconv.Itoa(current)
		}
	// More independent cases and conditions push the score over 10.
	}
	return ""
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
