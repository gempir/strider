---
title: spaced-compiler-directive
description: Detect compiler directives disabled by leading whitespace.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Purpose: detect top-level compiler directives that look intentional but are
ignored because whitespace appears between `//` and `go:`.

## Bad

```go
// go:noinline
func call() {}
```

## Good

```go
//go:noinline
func call() {}
```
