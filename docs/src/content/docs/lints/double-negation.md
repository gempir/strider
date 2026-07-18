---
title: double-negation
description: Remove redundant double boolean negation.
sidebar:
  badge:
    text: note
    class: severity-indicator severity-note
---

**Default severity:** <span class="severity-indicator severity-note" aria-hidden="true"></span> `note`

Negating a boolean twice produces the original value. The expression is either
redundant or contains a mistaken extra `!`.

```go
return !!ready // reported
return ready   // accepted
return !ready  // accepted when inversion is intended
```
