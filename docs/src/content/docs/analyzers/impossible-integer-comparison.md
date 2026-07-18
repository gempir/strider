---
title: impossible-integer-comparison
description: Detect integer comparisons fixed by the type's range.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

An integer type's minimum and maximum values make some comparisons always
true or false, such as checking whether an unsigned value is below zero or
above its maximum. Target-sized `int`, `uint`, and `uintptr` use the loaded
build architecture.

```go
value < 0  // reported when value is unsigned
value == 0 // accepted
```
