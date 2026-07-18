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

Strider default: available in the optional check catalog at `warning` severity.

```go
// go:noinline
func calculate() {} // reported: the directive is ignored

//go:noinline
func calculateExactly() {} // accepted
```
