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

## Good

```go
import "log/slog"
```

## Configuration

| Option | Type | Default | Description |
| --- | --- | --- | --- |
| `blocked-imports` | `strings` | `[]` | Import paths that this check rejects. |

```toml
[checks.imports-blocklist]
blocked-imports = []
```
