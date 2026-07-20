---
title: max-parameters
description: Limit declared functions to eight parameters.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Reports function declarations with more than eight parameters. Long parameter
lists are difficult to call correctly and often reveal a missing domain type.
Method receivers are not counted.

Each named parameter counts individually, including names that share a type.
An unnamed parameter field counts as one.

## Bad

```go
func Open(path string, read, write, create, truncate, appendMode, sync, exclusive, temporary bool) error {
	// ...
}
```

The example has nine parameters: `path` plus eight named booleans.

## Good

```go
type OpenOptions struct {
	Read       bool
	Write      bool
	Create     bool
	Truncate   bool
	AppendMode bool
}

func Open(path string, options OpenOptions) error {
	// ...
}
```

Prefer a cohesive options or request type. Do not combine unrelated values
into a struct purely to bypass the rule.

## Configuration

The default maximum is eight.

```toml
[checks.max-parameters]
max-parameters = 10
```

Set `max-parameters = 0` to use the built-in maximum of eight.
