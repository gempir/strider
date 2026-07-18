---
title: nil-map-assignment
description: Detect assignments into maps proven to be nil.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

Reading from a nil map is allowed, but assigning an entry to a nil map panics.
Initialize the map with `make` or a map literal before writing.

```go
var values map[string]int
values[key] = value // reported

values := make(map[string]int)
values[key] = value // accepted
```
