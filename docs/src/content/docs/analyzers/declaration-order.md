---
title: declaration-order
description: Keep top-level declarations in a consistent order.
sidebar:
  badge:
    text: note
    class: severity-indicator severity-note
---

**Default severity:** <span class="severity-indicator severity-note" aria-hidden="true"></span> `note`

Files are easier to scan when top-level declarations appear as types, constants,
variables, then functions. Imports are ignored, and `init` remains part of the
function group.

```go
type Client struct{}
const timeout = time.Second
var defaultClient Client
func New() Client { return Client{} }
```
