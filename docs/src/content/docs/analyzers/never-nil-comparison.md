---
title: never-nil-comparison
description: Detect nil checks on values proven to be non-nil.
---

**Default severity:** `error`

Fresh allocations, `make` results, functions, closures, and values flowing
exclusively from those sources cannot be nil. Comparing them with nil has a
fixed result and often means the wrong value was checked.

```go
values := make([]int, 0)
if values == nil { // reported
    unreachable()
}

var values []int
if values == nil { // accepted
    initialize()
}
```
