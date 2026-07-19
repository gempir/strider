---
title: function-length
description: "Limit function statements and lines."
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Limit function statements and lines.

## Bad

```go
func run() { /* more than 50 statements or 75 lines */ }
```

## Good

```go
func run() { load(); process(); save() }
```

## Configuration

The defaults are 50 statements and 75 lines.

```toml
[checks.rules.function-length]
max-statements = 60
max-lines = 100
```
