---
title: address-nil-comparison
description: Detect comparisons between a freshly taken address and nil.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Taking the address of an addressable value produces a non-nil pointer whenever
evaluation completes, so comparing that address with `nil` has a fixed result.

The `&*pointer` form is excluded because it simplifies to `pointer`, which may
legitimately be nil.

## Bad

```go
if &value == nil { handle() }
```

## Good

```go
if pointer == nil { handle() }
```
