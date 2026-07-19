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

The defaults are 50 statements and 75 lines.

```toml
[checks.rules.function-length]
max-statements = 60
max-lines = 100
```

Set either option to `0` to use its built-in default.
