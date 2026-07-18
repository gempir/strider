---
title: defer-close-before-error-check
description: Detect deferred Close calls scheduled before checking acquisition errors.
---

**Default severity:** 🔴 `error`

A resource-returning call may yield an unusable or nil value when it also
returns an error. Check the error before deferring `Close` on the resource.

```go
file, err := os.Open(path)
defer file.Close() // reported
if err != nil {
    return err
}

file, err := os.Open(path)
if err != nil {
    return err
}
defer file.Close() // accepted
```
