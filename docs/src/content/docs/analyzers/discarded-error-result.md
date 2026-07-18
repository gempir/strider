---
title: discarded-error-result
description: Detect discarded error results from typed calls.
---

**Default severity:** 🔴 `error`

This check uses resolved call signatures, so it catches arbitrary functions and
methods that return `error`, including errors assigned to `_`.

```go
value, _ := load() // reported

value, err := load()
if err != nil {
    return err
}
```
