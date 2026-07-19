---
title: constant-negative-zero
description: Detect constant expressions that cannot represent negative zero.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Go's ideal constants do not preserve a zero sign. Literal forms such as `-0.0`,
`-float64(0)`, and `float32(-0)` therefore produce positive zero at runtime.
Use `math.Copysign` when a true IEEE negative zero is required.

## Bad

```go
negativeZero := -0.0
```

## Good

```go
negativeZero := math.Copysign(0, -1)
```
