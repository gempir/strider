---
title: discarded-deferred-result
description: Detect deferred function results that are always discarded.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

The return values of a deferred call cannot be observed. A deferred function
literal that declares results usually contains a mistaken signature or should
store its result through another mechanism.

## Bad

```go
defer func() error {
	return cleanup()
}()
```

## Good

```go
defer func() {
	if err := cleanup(); err != nil {
		log.Printf("cleanup: %v", err)
	}
}()
```
