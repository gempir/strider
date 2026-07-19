---
title: invalid-strconv-argument
description: Detect invalid constant arguments to strconv functions.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

The `strconv` parsing and formatting functions accept only documented number
bases, bit sizes, and floating-point format characters. Invalid constant
arguments always return errors or produce unusable results.

## Bad

```go
strconv.ParseInt(value, 1, 128)
```

## Good

```go
strconv.ParseInt(value, 10, 64)
```
