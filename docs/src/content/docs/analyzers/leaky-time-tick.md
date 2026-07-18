---
title: leaky-time-tick
description: Detect time.Tick calls that leak on older Go versions.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Before Go 1.23, an unreferenced ticker could not be reclaimed unless it was
stopped. Because `time.Tick` does not expose the ticker, returning functions
should use `time.NewTicker` and stop it when they are finished.

Go 1.23 and newer can reclaim unreferenced tickers, so this check does not
report projects targeting those versions. Tests, `main` packages, and endless
functions are also accepted.

```go
ticks := time.Tick(time.Second) // reported when targeting Go 1.22 or older

ticker := time.NewTicker(time.Second)
defer ticker.Stop() // accepted
```
