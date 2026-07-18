---
title: max-parameters
description: Limit declared functions to five parameters.
---

**Default severity:** 🔵 `note`

**Maximum:** `5`  
**Configuration:** `enabled`, `severity`, and path `excludes`; maximum remains `5`

Reports function declarations with more than five parameters. Long parameter
lists are difficult to call correctly and often reveal a missing domain type.
Method receivers are not counted.

Each named parameter counts individually, including names that share a type.
An unnamed parameter field counts as one.

## Bad

```go
func Open(path string, read, write, create, truncate, appendMode bool) error {
	// ...
}
```

The example has seven parameters: `path` plus six named booleans.

## Good

```go
type OpenOptions struct {
	Read       bool
	Write      bool
	Create     bool
	Truncate   bool
	AppendMode bool
}

func Open(path string, options OpenOptions) error {
	// ...
}
```

Prefer a cohesive options or request type. Do not combine unrelated values
into a struct purely to bypass the rule.

## Suppress

```go
//strider:ignore max-parameters
func AdapterSignature(a, b, c, d, e, f string) {}
```
