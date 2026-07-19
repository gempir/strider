---
title: nan-comparison
description: Detect direct comparisons with NaN.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

IEEE floating-point NaN is unequal to every value, including itself, and all
ordered comparisons with it are false. Use `math.IsNaN` when testing whether a
value is NaN.

## Bad

```go
if value == math.NaN() { handle() }
```

## Good

```go
if math.IsNaN(value) { handle() }
```
