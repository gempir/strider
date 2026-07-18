---
title: waitgroup-add-inside-goroutine
description: Detect WaitGroup.Add calls inside newly started goroutines.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

`WaitGroup.Add` must happen before starting the goroutine it accounts for.
Calling `Add` inside the goroutine races with `Wait`, which may observe a zero
counter and return too early.

```go
go func() { group.Add(1); defer group.Done() }() // reported

group.Add(1)
go func() { defer group.Done() }()               // accepted
```
