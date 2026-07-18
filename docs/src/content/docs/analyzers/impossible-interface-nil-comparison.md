---
title: impossible-interface-nil-comparison
description: Detect interface comparisons made non-nil by a concrete dynamic type.
---

**Default severity:** 🔴 `error`

An interface is nil only when both its dynamic type and value are absent.
Storing a typed nil pointer in an interface gives it a concrete dynamic type,
so the interface itself is non-nil.

```go
func result() error {
    var problem *Problem
    return problem // produces a non-nil error interface
}

if result() == nil { // reported as never true
    handleSuccess()
}
```

Return an explicit `nil` interface on the success path instead.
