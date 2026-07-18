---
title: partially-typed-constant-group
description: Detect constant groups where only the first explicit value has a type.
---

**Default severity:** 🔵 `note`

In a constant group, an explicit type is inherited only when a later
declaration omits its value. If every declaration has an explicit literal but
only the first has a type, later constants silently use default built-in types
and may lose methods or assignment compatibility.

```go
const (
    first Kind = 1
    second = 2 // reported: defaults to int
)

const (
    first Kind = iota
    second // accepted: repeats the complete previous expression and type
)
```
