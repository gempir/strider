---
title: slog-argument-shape
description: Detect malformed or inconsistent log/slog arguments.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Reports odd key/value tails, non-string loose keys, and calls that mix
`slog.Attr` values with loose key/value pairs.

## Bad

```go
slog.Info("request", 42, method, "status")
```

## Good

```go
slog.Info("request", "method", method, "status", status)
```
