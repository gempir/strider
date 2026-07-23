---
title: function-length
description: "Limit function statements and lines."
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Reports a function that exceeds either the statement or line limit. Nested
statements count toward the total, and the line span includes the declaration
and braces.

The compact examples below assume `max-statements = 3`.

## Bad

```go
func run() {
	load()
	process()
	save()
	notify()
}
```

## Good

```go
func run() {
	load()
	processAndSave()
	notify()
}
```

## Configuration

| Option | Type | Default | Description |
| --- | --- | --- | --- |
| `max-lines` | `int` | `75` | Maximum number of lines allowed in a function. |
| `max-statements` | `int` | `50` | Maximum number of statements allowed in a function. |

```toml
[checks.function-length]
max-lines = 75
max-statements = 50
```
