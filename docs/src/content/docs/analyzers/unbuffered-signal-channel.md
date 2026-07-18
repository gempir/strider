---
title: unbuffered-signal-channel
description: Detect unbuffered channels used for signal notification.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

The `os/signal` package uses non-blocking sends to deliver notifications. If
no receiver is immediately ready, an unbuffered channel drops the signal.
Give the channel enough capacity for the signals being handled.

```go
ch := make(chan os.Signal)    // reported when passed to signal.Notify
ch := make(chan os.Signal, 1) // accepted for one signal value
```
