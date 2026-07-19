---
title: pointless-integer-math
description: Detect floating-point helpers applied to converted integers.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

An integer converted to floating point is already integral and finite.
Rounding it with `math.Ceil`, `math.Floor`, or `math.Trunc`, or testing it with
`math.IsNaN` or `math.IsInf`, cannot provide useful information.

## Bad

```go
rounded := math.Ceil(float64(count))
```

## Good

```go
rounded := math.Ceil(measurement)
```
