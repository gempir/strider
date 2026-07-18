---
title: swapped-errors-is-arguments
description: Detect likely reversed errors.Is arguments.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

`errors.Is` expects the error being inspected first and the target sentinel
second. A package-level sentinel from another package in the first position,
followed by a local error value, usually means the arguments were reversed.

```go
errors.Is(io.EOF, err) // reported
errors.Is(err, io.EOF) // accepted
```
