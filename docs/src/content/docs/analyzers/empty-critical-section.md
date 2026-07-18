---
title: empty-critical-section
description: Detect adjacent lock and unlock calls.
---

**Default severity:** 🟡 `warning`

A lock immediately followed by its matching unlock protects no work and is
commonly a missing `defer`. Intentional empty critical sections used for
synchronization should be documented and suppressed explicitly.

```go
mutex.Lock()
mutex.Unlock()       // reported

mutex.Lock()
defer mutex.Unlock() // accepted
```
