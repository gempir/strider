---
title: time-value-equality
description: Compare time.Time values with Time.Equal.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

The `==` and `!=` operators compare every field of `time.Time`, including
monotonic-clock and location representation details. `Time.Equal` compares the
represented instants.

## Bad

```go
if first == second {
	match()
}
```

## Good

```go
if first.Equal(second) {
	match()
}
```
