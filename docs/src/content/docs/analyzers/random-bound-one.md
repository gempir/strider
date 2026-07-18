---
title: random-bound-one
description: Detect random integer calls whose upper bound permits only zero.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Bounded random integer functions generate values in the half-open range from
zero up to, but excluding, the bound. A bound of one therefore always returns
zero.

```go
choice := rand.Intn(1) // reported: always zero
choice := rand.Intn(2) // accepted: zero or one
```

The check covers package functions and `Rand` methods in both standard
random packages.
