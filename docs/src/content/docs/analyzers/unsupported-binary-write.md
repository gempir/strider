---
title: unsupported-binary-write
description: Detect values encoding/binary cannot serialize.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

Reports architecture-sized integers and other variable-size values passed to
`binary.Write`.

```go
binary.Write(writer, binary.LittleEndian, value) // reported when value is int
```
