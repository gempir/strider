---
title: case-insensitive-string-comparison
description: Detect allocating case conversions used only for comparison.
---

**Default severity:** `warning`

Converting both strings with `strings.ToLower` or `strings.ToUpper` allocates
intermediate strings and processes each input fully. `strings.EqualFold`
compares incrementally without those allocations and can stop at the first
mismatch.

```go
strings.ToLower(left) == strings.ToLower(right) // reported
strings.EqualFold(left, right)                  // accepted
```
