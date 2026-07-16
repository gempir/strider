---
title: byte-string-write
description: Detect byte slices converted to strings for io.WriteString.
---

**Default severity:** `warning`

`io.WriteString(writer, string(bytes))` allocates and copies the byte slice
before writing it. The writer already accepts bytes through `Write`, so write
the original slice directly.

```go
io.WriteString(writer, string(bytes)) // reported
writer.Write(bytes)                   // accepted
```
