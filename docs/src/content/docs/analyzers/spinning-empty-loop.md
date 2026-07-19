---
title: spinning-empty-loop
description: Detect empty loops that consume a core while waiting unsafely.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

An empty unconditional loop spins at full speed. An empty loop that only
rereads variables can terminate only through unsynchronized mutation, which is
a data race. Use synchronization or a blocking operation instead.

Conditions containing calls or channel receives are accepted because their
result can change dynamically. Constant-false loops are accepted as disabled
debug scaffolding.

## Bad

```go
for {}
```

## Good

```go
for !ready() {} // condition is dynamically evaluated
```
