---
title: unused-append-result
description: Detect append results that can never be observed.
---

**Default severity:** `warning`

`append` returns the updated slice header. Discarding that result loses any new
length or reallocated backing array. The check reports only function-local
slices whose backing storage has not escaped or been observably aliased.

```go
values := make([]int, 0, 1)
values = append(values, item) // reported when values is never read again

values = append(values, item)
return values // accepted
```
