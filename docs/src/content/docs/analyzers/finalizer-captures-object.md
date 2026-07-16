---
title: finalizer-captures-object
description: Detect finalizers that retain the object they should release.
---

**Default severity:** `warning`

A finalizer closure that captures the finalized object keeps that object
reachable. The garbage collector can never make the object eligible for
finalization, so the finalizer never runs and the object leaks.

Use the finalizer function's parameter to operate on the object instead of
capturing the outer variable.

```go
runtime.SetFinalizer(object, func(*resource) {
    object.Close() // reported: captures the outer object
})

runtime.SetFinalizer(object, func(object *resource) {
    object.Close() // accepted: uses the finalizer parameter
})
```
