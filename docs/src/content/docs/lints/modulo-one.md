---
title: modulo-one
description: Detect remainder operations that are always zero.
---

**Default severity:** `warning`

Every integer is evenly divisible by one, so `value % 1` always produces zero.
The expression usually contains a mistaken divisor.

```go
remainder := value % 1 // reported
remainder := value % 2 // accepted
```
