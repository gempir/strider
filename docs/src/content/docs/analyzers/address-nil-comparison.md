---
title: address-nil-comparison
description: Detect comparisons between a freshly taken address and nil.
---

**Default severity:** `warning`

Taking the address of an addressable value produces a non-nil pointer whenever
evaluation completes, so comparing that address with `nil` has a fixed result.

The `&*pointer` form is excluded because it simplifies to `pointer`, which may
legitimately be nil.

```go
if &value == nil { // reported
    handle()
}

if pointer == nil { // accepted
    handle()
}
```
