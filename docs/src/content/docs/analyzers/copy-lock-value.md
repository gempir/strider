---
title: copy-lock-value
description: Detect values that copy sync locks.
---

**Default severity:** 🔴 `error`

Copying a value that contains `sync.Mutex` or `sync.RWMutex` creates an
independent lock state and can invalidate the intended synchronization. Pass
such values by pointer and avoid value receivers, assignments, and returns that
copy them.
