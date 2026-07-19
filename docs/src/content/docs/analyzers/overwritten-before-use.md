---
title: overwritten-before-use
description: Detect assigned values that are replaced before being used.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

A non-constant value assigned to a local variable but never read is often a
forgotten error check or dead computation. The check follows values through
control-flow joins and separately tracks each result of multi-value calls.

## Bad

```go
result := calculate(); result = calculateAgain()
```

## Good

```go
result := calculate(); use(result); result = calculateAgain()
```
