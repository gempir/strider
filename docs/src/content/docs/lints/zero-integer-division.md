---
title: zero-integer-division
description: Detect literal integer division that truncates to zero.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

Dividing two integer literals uses integer arithmetic. A fraction such as
`2 / 3` therefore truncates to zero, even when it is later converted or assigned
to a floating-point value.

Named constants are accepted to avoid warning on deliberate integer formulas.

```go
ratio := 2 / 3          // reported: result is zero
ratio := 2.0 / 3        // accepted: floating-point division
wholeAndRemainder := 4 / 3 // accepted: result is non-zero
```
