---
title: benchmark-iteration-mutation
description: Detect assignments to testing.B.N.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

The testing package dynamically controls `B.N` to calibrate benchmark
duration and calculate per-operation time. Benchmark code that changes `N`
invalidates those measurements.

## Bad

```go
b.N = 1000
```

## Good

```go
for range b.N { operation() }
```
