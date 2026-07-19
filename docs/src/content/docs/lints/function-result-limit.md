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

## Configuration

The default maximum is three results.

```toml
[checks.rules.function-result-limit]
max-results = 4
```

## Good

```go
func Parse() (Value, error)
```
