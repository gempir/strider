---
title: duration-multiplied-by-duration
description: Detect multiplication of two time.Duration values.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

`time.Duration` is a scalar count of nanoseconds. Multiplying two duration
values produces squared time units, which is almost never meaningful. This
often happens when a value that was expected to be a unitless count has already
been converted to a duration.

## Bad

```go
delay := duration * time.Second
```

## Good

```go
delay := time.Duration(count) * time.Second
```
