---
title: spinning-select-default
description: Detect select loops that spin on an empty default.
---

**Default severity:** 🟡 `warning`

An empty `default` makes a `select` immediately ready. Inside an unconditional
loop, this prevents the goroutine from blocking and consumes CPU continuously.
Remove the empty default so the select can wait for a communication case.

```go
for {
    select {
    case value := <-values:
        use(value)
    default: // reported when empty
    }
}
```
