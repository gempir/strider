---
title: zero-replacement-limit
description: Detect replacement calls with a zero limit.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

The final argument to `strings.Replace` and `bytes.Replace` is the maximum
number of replacements. Zero replaces nothing. Use a negative value or the
corresponding `ReplaceAll` function to replace every occurrence.

```go
strings.Replace(value, old, replacement, 0) // reported
strings.ReplaceAll(value, old, replacement) // accepted
```
