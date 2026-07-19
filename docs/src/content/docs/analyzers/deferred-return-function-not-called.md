---
title: deferred-return-function-not-called
description: Detect deferred setup calls whose returned function is not called.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Functions sometimes perform setup immediately and return a function that
performs cleanup. Deferring only the first call delays setup until function
exit and then discards the returned cleanup function.

## Bad

```go
defer setup()
```

## Good

```go
defer setup()()
```
