---
title: range-value-capture
description: Detect closures that capture reused range variables.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Before Go 1.22, variables declared by a range clause were reused across
iterations. Variables assigned with `=` are still reused on every Go version.
A closure that outlives an iteration can therefore observe a later value.
Immediately invoked closures are accepted.

## Bad

```go
var value int
for _, value = range values {
	callbacks = append(callbacks, func() { use(value) })
}
```

## Good

```go
for _, value := range values {
	go func(current int) {
		use(current)
	}(value)
}
```
