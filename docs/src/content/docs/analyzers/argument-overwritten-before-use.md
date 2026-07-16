---
title: argument-overwritten-before-use
description: Detect arguments replaced before their incoming value is used.
---

**Default severity:** `warning`

Overwriting a function argument before reading its incoming value makes that
input meaningless. The assignment may be accidental, or the argument may no
longer belong in the function signature.

```go
func normalize(value string) string {
    value = fallback // reported
    return value
}

func normalize(value string) string {
    use(value)
    value = fallback // accepted
    return value
}
```
