---
title: benchmark-iteration-mutation
description: Detect assignments to testing.B.N.
---

**Default severity:** 🔴 `error`

The testing package dynamically controls `B.N` to calibrate benchmark
duration and calculate per-operation time. Benchmark code that changes `N`
invalidates those measurements.

```go
b.N = 1000        // reported
for range b.N {}  // accepted
```
