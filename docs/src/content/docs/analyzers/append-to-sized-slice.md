---
title: append-to-sized-slice
description: Detect appends to slices created with a known positive length.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

`make([]T, n)` with a compile-time positive `n` creates `n` existing zero
values. If the intent is to append up to `n` values, use `make([]T, 0, n)` so
the result does not begin with zeros. Runtime lengths are left alone when the
analyzer cannot prove that they are positive.
