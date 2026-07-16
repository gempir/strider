---
title: self-assignment
description: Detect assignments that store an expression back into itself.
---

**Default severity:** `warning`

Assigning a side-effect-free expression to the identical destination does
nothing and usually indicates a mistaken variable on one side. Expressions
with effectful calls or channel receives are excluded because evaluating them
can change state.

```go
current = current // reported
current = next    // accepted

values[next()] = values[next()] // accepted when next may have side effects
```
