---
title: argument-overwritten-before-use
description: Detect arguments replaced before their incoming value is used.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Overwriting a function argument before reading its incoming value makes that
input meaningless. The assignment may be accidental, or the argument may no
longer belong in the function signature.

## Bad

```go
func normalize(value string) string { value = fallback; return value }
```

## Good

```go
func normalize(value string) string { use(value); value = fallback; return value }
```
