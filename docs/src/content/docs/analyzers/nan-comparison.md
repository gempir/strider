---
title: nan-comparison
description: Detect direct comparisons with NaN.
---

**Default severity:** `error`

IEEE floating-point NaN is unequal to every value, including itself, and all
ordered comparisons with it are false. Use `math.IsNaN` when testing whether a
value is NaN.

```go
if value == math.NaN() { // reported
    handle()
}

if math.IsNaN(value) { // accepted
    handle()
}
```
