---
title: writer-buffer-mutation
description: Detect io.Writer implementations that modify their input buffer.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

The `io.Writer` contract requires `Write` implementations not to modify the
provided byte slice, even temporarily. Mutating an element or appending into
the input can corrupt caller-owned data.

## Bad

```go
func (w *writer) Write(p []byte) (int, error) { p[0] = 0; return len(p), nil }
```

## Good

```go
func (w *writer) Write(p []byte) (int, error) { return w.dst.Write(p) }
```
