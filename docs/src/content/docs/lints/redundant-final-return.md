---
title: redundant-final-return
description: Remove a bare return at the end of a resultless function.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

A resultless function returns automatically after its final statement. An
explicit bare return at that position adds no control-flow information.

## Bad

```go
func notify() {
	sendNotification()
	return
}
```

## Good

```go
func notify() {
	sendNotification()
}
```
