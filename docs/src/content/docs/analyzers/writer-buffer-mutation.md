---
title: writer-buffer-mutation
description: Detect io.Writer implementations that modify their input buffer.
---

**Default severity:** 🔴 `error`

The `io.Writer` contract requires `Write` implementations not to modify the
provided byte slice, even temporarily. Mutating an element or appending into
the input can corrupt caller-owned data.

```go
buffer[0] = 0                         // reported inside Write
copyOfBuffer := append([]byte{}, buffer...) // accepted
```
