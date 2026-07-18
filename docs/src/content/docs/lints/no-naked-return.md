---
title: no-naked-return
description: Require explicit values when returning from named-result functions.
---

**Default severity:** 🔵 `note`

**Configuration:** `enabled`, `severity`, and path `excludes`

Reports a bare `return` when the nearest function declaration or function
literal has at least one named result. Naked returns make data flow implicit:
the reader must search backward to determine which values are returned.

## Bad

```go
func value() (result int) {
	result = calculate()
	return
}
```

## Good

```go
func value() (result int) {
	result = calculate()
	return result
}
```

The rule does not require removing named results. It only requires the return
statement to name the values being returned.

## Suppress

```go
//strider:ignore no-naked-return
func tinyWrapper() (result int) {
	result = 1
	return
}
```
