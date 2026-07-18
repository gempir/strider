---
title: single-argument-append
description: Detect append calls that add no elements.
---

**Default severity:** `note`

Calling the predeclared `append` function with only a slice argument returns
that same slice unchanged. Assign or return the slice directly instead.

```go
destination = append(source)        // reported
destination = source                // accepted
destination = append(source, value) // accepted
```
