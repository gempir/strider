---
title: top-level-declaration-order
description: Keep top-level declarations in a consistent order.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Files are easier to scan when top-level declarations appear as constants,
variables, types, then functions. Imports are ignored, and `init` remains part of the
function group.

## Bad

```go
type Client struct{}
const timeout = time.Second
```

## Good

```go
const timeout = time.Second
var defaultClient Client
type Client struct{}
func New() Client { return Client{} }
```
