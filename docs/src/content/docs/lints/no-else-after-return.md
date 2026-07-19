---
title: no-else-after-return
description: Remove unnecessary nesting after a terminal return.
sidebar:
  badge:
    text: note
    class: severity-indicator severity-note
---

**Default severity:** <span class="severity-indicator severity-note" aria-hidden="true"></span> `note`

**Configuration:** `severity` and path `excludes`

Reports an `else` when the final direct statement of the corresponding `if`
body is a `return`. Since control flow cannot continue past that return, the
else body can be moved out one indentation level.

## Bad

```go
if err != nil {
	return err
} else {
	use(value)
}
```

## Good

```go
if err != nil {
	return err
}
use(value)
```

The current rule checks for a direct final `return`. It does not treat `panic`,
`continue`, `break`, or a helper that never returns as equivalent terminals.

## Suppress

```go
//strider:ignore no-else-after-return
if err != nil {
	return err
} else {
	// Deliberately mirrors a two-column protocol specification.
	use(value)
}
```
