---
title: double-negation
description: Remove redundant double boolean negation.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

Negating a boolean twice produces the original value. The expression is either
redundant or contains a mistaken extra `!`.

```go
return !!ready // reported
return ready   // accepted
return !ready  // accepted when inversion is intended
```
