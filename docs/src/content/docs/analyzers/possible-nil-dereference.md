---
title: possible-nil-dereference
description: Detect pointer dereferences not protected by their nil checks.
---

**Default severity:** 🔴 `error`

Checking a pointer against nil is evidence that nil is a possible value. A
dereference that is not dominated by the check's non-nil path may panic. This
commonly happens when the dereference precedes the check, or when the nil
branch logs an error but continues execution.

The check follows control-flow dominance so guarded dereferences and code
reached only after a terminating nil branch are accepted.

```go
if value == nil {
    reportError()
}
use(*value) // reported

if value == nil {
    return
}
use(*value) // accepted
```
