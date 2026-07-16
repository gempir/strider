---
title: negative-length-capacity-comparison
description: Detect checks for negative len or cap results.
---

**Default severity:** `warning`

The predeclared `len` and `cap` functions always return non-negative values, so
testing whether either result is below zero can never succeed.

```go
if len(values) < 0 { // reported
    unreachable()
}

if len(values) == 0 { // accepted
    handleEmpty()
}
```
