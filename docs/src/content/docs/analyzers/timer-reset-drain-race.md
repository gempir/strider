---
title: timer-reset-drain-race
description: Detect attempts to drain a timer based on Reset's result.
---

**Default severity:** `error`

Using `time.Timer.Reset`'s boolean result to decide whether to receive from the
timer channel is racy on older timer implementations and can block with
current synchronous timer channels. Stop and drain before resetting when
compatibility requires it, or reset without conditionally draining afterward.

```go
if !timer.Reset(delay) { <-timer.C } // reported
timer.Reset(delay)                    // accepted
```
