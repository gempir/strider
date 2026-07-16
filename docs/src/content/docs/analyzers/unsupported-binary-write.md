---
title: unsupported-binary-write
description: Detect values encoding/binary cannot serialize.
---

**Default severity:** `error`

Reports architecture-sized integers and other variable-size values passed to
`binary.Write`.

```go
binary.Write(writer, binary.LittleEndian, value) // reported when value is int
```
