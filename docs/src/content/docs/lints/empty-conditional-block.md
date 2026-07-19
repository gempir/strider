---
title: empty-conditional-block
description: Detect empty statement blocks that need removal or explanation.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

Catch empty `if` and `else` branches that are usually unfinished code
or accidental no-ops.

## Behavior

Empty functions, methods, closures, and loop bodies are accepted because they
are commonly intentional stubs, marker methods, callbacks, or drain loops and
have different semantics from an empty conditional branch.

## Bad

```go
if err != nil {
}
```

## Good

```go
if err != nil {
	return err
}
```
