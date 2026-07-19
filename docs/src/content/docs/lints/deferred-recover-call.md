---
title: deferred-recover-call
description: Defer a function that calls recover instead of deferring recover directly.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

`recover` only stops a panic when it is called from a deferred function. A
direct `defer recover()` evaluates `recover` while scheduling the defer, so it
cannot recover the later panic.

## Bad

```go
defer recover()
```

## Good

```go
defer func() {
	_ = recover()
}()
```
