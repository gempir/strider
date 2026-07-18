---
title: single-argument-append
description: Detect append calls that add no elements.
sidebar:
  badge:
    text: note
    class: severity-indicator severity-note
---

**Default severity:** <span class="severity-indicator severity-note" aria-hidden="true"></span> `note`

Calling the predeclared `append` function with only a slice argument returns
that same slice unchanged. Assign or return the slice directly instead.

```go
destination = append(source)        // reported
destination = source                // accepted
destination = append(source, value) // accepted
```
