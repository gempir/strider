---
title: oversized-fixed-width-shift
description: Detect shifts that always clear fixed-width integers.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Shifting a fixed-width integer by its full width or more always clears every
value bit. This is usually an incorrect shift count. Machine-sized `int`,
`uint`, and `uintptr` are excluded because width-dependent bit manipulation can
be intentional.

## Bad

```go
value := uint8(1) << 8
```

## Good

```go
value := uint8(1) << 7
```
