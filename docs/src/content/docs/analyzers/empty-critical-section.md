---
title: empty-critical-section
description: Detect adjacent lock and unlock calls.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

A lock immediately followed by its matching unlock protects no work and is
commonly a missing `defer`. Intentional empty critical sections used for
synchronization should be documented and suppressed explicitly.

## Bad

```go
mutex.Lock()
mutex.Unlock()
```

## Good

```go
mutex.Lock()
defer mutex.Unlock()
```
