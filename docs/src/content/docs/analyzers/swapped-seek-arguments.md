---
title: swapped-seek-arguments
description: Detect swapped io.Seeker.Seek arguments.
---

**Default severity:** `warning`

The byte offset is the first argument and the whence constant is the second.

```go
seeker.Seek(io.SeekStart, 0) // reported
seeker.Seek(0, io.SeekStart) // accepted
```
