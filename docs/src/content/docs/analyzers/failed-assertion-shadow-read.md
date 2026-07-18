---
title: failed-assertion-shadow-read
description: Detect reads of a shadowing failed type assertion result.
---

**Default severity:** 🟡 `warning`

An `if` initializer such as `if value, ok := value.(T); ok` declares a new
`value` that remains in scope in the `else` branch. When the assertion fails,
that variable contains `T`'s zero value. Reading it in the failure branch often
means the original interface value was intended.

```go
if value, ok := value.(int); ok {
	use(value)
} else {
	logType(value) // reported: this is always the integer zero value
}

if typed, ok := value.(int); ok {
	use(typed)
} else {
	logType(value) // accepted: this is the original interface value
}
```
