---
title: redundant-switch-break
description: Remove an unlabeled break at the end of a switch case.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Go switch cases do not fall through unless they explicitly use `fallthrough`.
An unlabeled `break` as the final statement of a case is therefore redundant.

## Bad

```go
switch value {
case 1:
	use(value)
	break
}
```

## Good

```go
switch value {
case 1:
	use(value)
}
```
