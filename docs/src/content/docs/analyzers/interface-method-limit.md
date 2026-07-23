---
title: interface-method-limit
description: Detect interfaces with more than ten methods.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Counts explicit and embedded methods after completing the interface. More than
ten suggests an abstraction that may be easier to implement and test when
split by responsibility.

## Bad

```go
type Service interface {
	Start()
	Stop()
	Pause()
	Resume()
	Reload()
	Status()
	Health()
	Metrics()
	Configure()
	Validate()
	Reset()
}
```

## Good

```go
type Reader interface {
	Read([]byte) (int, error)
}
```

## Configuration

| Option | Type | Default | Description |
| --- | --- | --- | --- |
| `max-methods` | `int` | `10` | Maximum number of methods allowed on an interface, including embedded methods. |

```toml
[checks.interface-method-limit]
max-methods = 10
```
