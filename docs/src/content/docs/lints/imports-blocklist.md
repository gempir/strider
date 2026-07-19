---
title: imports-blocklist
description: "Reject configured imports."
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Reject configured imports.

## Bad

```go
import "log" // when log is configured as blocked
```

## Configuration

No imports are blocked by default. Paths match exactly.

```toml
[checks.rules.imports-blocklist]
blocked-imports = ["log", "io/ioutil"]
```

## Good

```go
import "log/slog"
```
