---
title: nil-map-assignment
description: Detect assignments into maps proven to be nil.
---

**Default severity:** `error`

Reading from a nil map is allowed, but assigning an entry to a nil map panics.
Initialize the map with `make` or a map literal before writing.

```go
var values map[string]int
values[key] = value // reported

values := make(map[string]int)
values[key] = value // accepted
```
