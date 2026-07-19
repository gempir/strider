---
title: unused-append-result
description: Detect append results that can never be observed.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

`append` returns the updated slice header. Discarding that result loses any new
length or reallocated backing array. The check reports only function-local
slices whose backing storage has not escaped or been observably aliased.

## Bad

```go
values := make([]int, 0); values = append(values, item) // values is never read again
```

## Good

```go
values = append(values, item)
```
