---
title: nil-context
description: Detect nil context.Context arguments.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

A context must not be nil. Use `context.TODO()` when the appropriate parent is
not known or `context.Background()` for an explicit root.

```go
load(nil) // reported when the first parameter is context.Context
```
