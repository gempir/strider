---
title: unreachable-type-switch-case
description: Detect type-switch cases hidden by earlier interfaces.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Type-switch cases are evaluated in source order. A later concrete type or
narrower interface is unreachable when it necessarily implements an interface
listed by an earlier case.

## Bad

```go
switch value.(type) { case io.Reader: use(); case io.ReadCloser: use() }
```

## Good

```go
switch value.(type) { case io.ReadCloser: use(); case io.Reader: use() }
```
