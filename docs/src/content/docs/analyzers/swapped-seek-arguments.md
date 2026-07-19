---
title: swapped-seek-arguments
description: Detect swapped io.Seeker.Seek arguments.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

The byte offset is the first argument and the whence constant is the second.

## Bad

```go
seeker.Seek(io.SeekStart, 0)
```

## Good

```go
seeker.Seek(0, io.SeekStart)
```
