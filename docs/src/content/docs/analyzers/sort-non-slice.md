---
title: sort-non-slice
description: Detect sort.Slice calls with non-slice values.
---

**Default severity:** 🔴 `error`

`sort.Slice`, `sort.SliceStable`, and `sort.SliceIsSorted` accept `any` only
for historical API reasons. The value must hold a slice; passing an array or
another concrete type panics at runtime.

```go
sort.Slice(array, less) // reported
sort.Slice(slice, less) // accepted
```
