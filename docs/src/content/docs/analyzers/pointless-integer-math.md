---
title: pointless-integer-math
description: Detect floating-point helpers applied to converted integers.
---

**Default severity:** 🟡 `warning`

An integer converted to floating point is already integral and finite.
Rounding it with `math.Ceil`, `math.Floor`, or `math.Trunc`, or testing it with
`math.IsNaN` or `math.IsInf`, cannot provide useful information.

```go
rounded := math.Ceil(float64(count)) // reported
rounded := math.Ceil(measurement)    // accepted
```
