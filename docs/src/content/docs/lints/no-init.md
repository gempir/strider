---
title: no-init
description: Avoid implicit package initialization.
sidebar:
  badge:
    text: note
    class: severity-indicator severity-note
---

**Default severity:** <span class="severity-indicator severity-note" aria-hidden="true"></span> `note`

**Configuration:** `severity` and path `excludes`

Reports package `init` functions. Initialization functions hide side effects
and ordering constraints from callers, which makes packages harder to test and
compose.

## Bad

```go
func init() {
	registerHandlers()
}
```

## Good

```go
func RegisterHandlers(registry *Registry) error {
	return registry.Register(defaultHandlers())
}
```

Call explicit setup from `main`, a constructor, or the test that needs it.
Explicit setup can return errors and makes dependencies visible.

## Suppress

```go
//strider:ignore no-init
func init() {
	// Required by an integration contract that cannot call setup explicitly.
}
```
