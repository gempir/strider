---
title: single-case-switch
description: Replace a switch with one simple case by an if statement.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

A switch with one non-default condition and no additional cases is more
directly expressed as an `if` statement.

## Bad

```go
switch value {
case 1:
	use(value)
}
```

## Good

```go
if value == 1 {
	use(value)
}
```
