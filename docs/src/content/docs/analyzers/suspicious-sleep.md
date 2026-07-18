---
title: suspicious-sleep
description: Detect suspiciously small bare time.Sleep literals.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Reports bare integer literals from 1 through 120 because `time.Sleep` treats
them as nanoseconds.

```go
time.Sleep(5) // reported
time.Sleep(5 * time.Second) // accepted
```
