---
title: deferred-lock-after-lock
description: Detect deferring Lock immediately after locking.
---

**Default severity:** `warning`

Deferring `Lock` or `RLock` immediately after acquiring the same lock is
almost always a typo for deferring `Unlock` or `RUnlock` and is likely to
deadlock when the function returns.

```go
mutex.Lock()
defer mutex.Lock()   // reported

mutex.Lock()
defer mutex.Unlock() // accepted
```
