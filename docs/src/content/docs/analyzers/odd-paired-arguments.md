---
title: odd-paired-arguments
description: Detect odd element counts passed to pair-oriented APIs.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

Some functions consume slice or variadic elements in pairs and reject odd
lengths. The check recognizes standard pair-oriented APIs and local
functions that enforce an even length by panicking, then checks calls whose
argument length is statically known.

## Bad

```go
strings.NewReplacer("old", "new", "orphan")
```

## Good

```go
strings.NewReplacer("old", "new")
```
