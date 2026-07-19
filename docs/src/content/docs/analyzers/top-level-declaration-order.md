---
title: top-level-declaration-order
description: Keep top-level declarations in a consistent order.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Files are easier to scan when top-level declarations appear as types, constants,
variables, then functions. Imports are ignored, and `init` remains part of the
function group.

## Bad

```go
var defaultClient Client
type Client struct{}
```

## Good

```go
type Client struct{}
const timeout = time.Second
var defaultClient Client
func New() Client { return Client{} }
```
