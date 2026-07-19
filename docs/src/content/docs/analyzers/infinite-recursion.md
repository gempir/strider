---
title: infinite-recursion
description: Detect recursive calls with no path to a function exit.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

A recursive call must have a path that reaches a function exit without making
that call. Otherwise recursion continues until the goroutine stack exhausts
available memory. Go does not optimize tail calls, so deliberate infinite
recursion should be written as a loop.

Recursively starting a new goroutine is not reported because it does not grow
one goroutine's stack indefinitely.

## Bad

```go
func visit() { visit() }
```

## Good

```go
if done { return }; visit(next)
```
