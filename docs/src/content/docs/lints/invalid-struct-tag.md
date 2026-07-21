---
title: invalid-struct-tag
description: Validate struct tag syntax, duplicate keys, and standard options.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

Detect malformed struct tags before reflection or encoding packages
silently ignore them.

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

## Behavior

The rule validates quoted key-value syntax, duplicate keys, whitespace and
quoting in common tag names, and JSON/XML options. Duplicate keys are reported
consistently regardless of which packages the file imports.

## Bad

```go
type User struct {
	Name string `json:name`
}
```

## Good

```go
type User struct {
	Name string `json:"name"`
}
```
