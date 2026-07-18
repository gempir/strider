---
title: overwritten-before-use
description: Detect assigned values that are replaced before being used.
---

**Default severity:** 🟡 `warning`

A non-constant value assigned to a local variable but never read is often a
forgotten error check or dead computation. The check follows values through
control-flow joins and separately tracks each result of multi-value calls.

```go
result := calculate()
result = calculateAgain() // the first result is reported

result := calculate()
use(result)
result = calculateAgain() // accepted
```
