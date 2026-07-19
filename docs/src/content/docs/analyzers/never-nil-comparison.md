---
title: never-nil-comparison
description: Detect nil checks on values proven to be non-nil.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

Fresh allocations, `make` results, functions, closures, and values flowing
exclusively from those sources cannot be nil. Comparing them with nil has a
fixed result and often means the wrong value was checked.

## Bad

```go
values := make([]int, 0); if values == nil { unreachable() }
```

## Good

```go
var values []int; if values == nil { initialize() }
```
