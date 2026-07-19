---
title: byte-string-write
description: Detect byte slices converted to strings for io.WriteString.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

`io.WriteString(writer, string(bytes))` allocates and copies the byte slice
before writing it. The writer already accepts bytes through `Write`, so write
the original slice directly.

## Bad

```go
_, err := io.WriteString(writer, string(bytes))
```

## Good

```go
_, err := writer.Write(bytes)
```
