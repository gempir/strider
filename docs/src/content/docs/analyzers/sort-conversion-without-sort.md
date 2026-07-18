---
title: sort-conversion-without-sort
description: Detect slice type conversions mistaken for sorting calls.
---

**Default severity:** 🟡 `warning`

`sort.Float64Slice`, `sort.IntSlice`, and `sort.StringSlice` are types, not
sorting functions. Converting a slice to one of these types and assigning it
back does not reorder any values.

```go
values = sort.IntSlice(values) // reported
sort.Ints(values)              // accepted
sort.Sort(sort.IntSlice(values)) // accepted
```
