---
title: sort-non-slice
description: Detect sort.Slice calls with non-slice values.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

`sort.Slice`, `sort.SliceStable`, and `sort.SliceIsSorted` accept `any` only
for historical API reasons. The value must hold a slice; passing an array or
another concrete type panics at runtime.

## Bad

```go
sort.Slice(array, less)
```

## Good

```go
sort.Slice(values, less)
```
