---
title: invalid-strconv-argument
description: Detect invalid constant arguments to strconv functions.
---

**Default severity:** `error`

The `strconv` parsing and formatting functions accept only documented number
bases, bit sizes, and floating-point format characters. Invalid constant
arguments always return errors or produce unusable results.

```go
strconv.ParseInt(value, 1, 64)  // reported
strconv.ParseInt(value, 10, 64) // accepted
```
