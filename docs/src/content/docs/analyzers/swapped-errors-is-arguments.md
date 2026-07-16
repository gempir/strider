---
title: swapped-errors-is-arguments
description: Detect likely reversed errors.Is arguments.
---

**Default severity:** `error`

`errors.Is` expects the error being inspected first and the target sentinel
second. A package-level sentinel from another package in the first position,
followed by a local error value, usually means the arguments were reversed.

```go
errors.Is(io.EOF, err) // reported
errors.Is(err, io.EOF) // accepted
```
