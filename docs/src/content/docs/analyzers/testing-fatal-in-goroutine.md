---
title: testing-fatal-in-goroutine
description: Detect test termination methods called from child goroutines.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

Methods on `testing.T` and `testing.B` that terminate or skip execution must
run in the same goroutine as the test. Calling `Fatal`, `FailNow`, `Skip`, or
related methods from a child goroutine does not stop the test correctly.

## Bad

```go
go func() { t.Fatal("failed") }()
```

## Good

```go
if err := work(); err != nil { t.Fatal(err) }
```
