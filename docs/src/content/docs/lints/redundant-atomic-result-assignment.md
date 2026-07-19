---
title: redundant-atomic-result-assignment
description: "Detect non-atomic operations on atomic values."
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

Reports an atomic update whose returned value is assigned back to the same
variable. The assignment performs a second, non-atomic write and defeats the
purpose of the atomic operation.

## Bad

```go
counter = atomic.AddInt64(&counter, 1)
```

## Good

```go
atomic.AddInt64(&counter, 1)
```
