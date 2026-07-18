---
title: duration-multiplied-by-duration
description: Detect multiplication of two time.Duration values.
---

**Default severity:** 🟡 `warning`

`time.Duration` is a scalar count of nanoseconds. Multiplying two duration
values produces squared time units, which is almost never meaningful. This
often happens when a value that was expected to be a unitless count has already
been converted to a duration.

```go
delay := duration * time.Second          // reported
delay := time.Duration(count) * time.Second // accepted
```
