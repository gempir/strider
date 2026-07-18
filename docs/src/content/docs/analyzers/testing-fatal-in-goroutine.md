---
title: testing-fatal-in-goroutine
description: Detect test termination methods called from child goroutines.
---

**Default severity:** 🔴 `error`

Methods on `testing.T` and `testing.B` that terminate or skip execution must
run in the same goroutine as the test. Calling `Fatal`, `FailNow`, `Skip`, or
related methods from a child goroutine does not stop the test correctly.

```go
go func() { t.Fatal("failed") }() // reported

if err != nil { t.Fatal(err) }    // accepted
```
