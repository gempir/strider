---
title: identical-binary-operands
description: Detect suspicious binary operations with identical operands.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Comparisons and non-idempotent operations with identical expressions on both
sides are usually copy-and-paste mistakes. Floating-point expressions are
excluded because `NaN` makes self-comparisons meaningful.

```go
value == value // reported for non-floating-point values
left == right  // accepted
```
