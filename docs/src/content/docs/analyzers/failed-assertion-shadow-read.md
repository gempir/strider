---
title: failed-assertion-shadow-read
description: Detect reads of a shadowing failed type assertion result.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

An `if` initializer such as `if value, ok := value.(T); ok` declares a new
`value` that remains in scope in the `else` branch. When the assertion fails,
that variable contains `T`'s zero value. Reading it in the failure branch often
means the original interface value was intended.

## Bad

```go
if value, ok := value.(T); ok { use(value) } else { logType(value) }
```

## Good

```go
if typed, ok := value.(T); ok { use(typed) } else { logType(value) }
```
