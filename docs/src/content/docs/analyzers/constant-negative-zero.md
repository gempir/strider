---
title: constant-negative-zero
description: Detect constant expressions that cannot represent negative zero.
---

**Default severity:** `warning`

Go's ideal constants do not preserve a zero sign. Literal forms such as `-0.0`,
`-float64(0)`, and `float32(-0)` therefore produce positive zero at runtime.
Use `math.Copysign` when a true IEEE negative zero is required.

```go
negativeZero := -0.0                  // reported
negativeZero := math.Copysign(0, -1) // accepted
```
