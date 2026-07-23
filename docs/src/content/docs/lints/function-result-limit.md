---
title: function-result-limit
description: "Limit function result count."
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Limit function result count.

## Bad

```go
func Parse() (Value, Metadata, Warnings, error)
```

## Good

```go
func Parse() (Value, error)
```

## Configuration

| Option | Type | Default | Description |
| --- | --- | --- | --- |
| `max-results` | `int` | `3` | Maximum number of result values allowed on a function. |

```toml
[checks.function-result-limit]
max-results = 3
```
