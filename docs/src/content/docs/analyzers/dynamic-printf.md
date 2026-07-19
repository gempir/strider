---
title: dynamic-printf
description: Detect printf-style calls with a lone dynamic format.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Reports supported printf-style calls whose only format argument is dynamic.
Use a print-style function or an explicit `%s` format.

## Bad

```go
fmt.Printf(message)
```

## Good

```go
fmt.Printf("%s", message)
```
