---
title: invalid-time-layout
description: Detect invalid time.Parse layouts.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

Validates constant layouts against Go's reference-time convention.

```go
time.Parse("YYYY-MM-DD", value) // reported
time.Parse("2006-01-02", value) // accepted
```
