---
title: odd-paired-arguments
description: Detect odd element counts passed to pair-oriented APIs.
---

**Default severity:** `error`

Some functions consume slice or variadic elements in pairs and reject odd
lengths. The analyzer recognizes standard pair-oriented APIs and local
functions that enforce an even length by panicking, then checks calls whose
argument length is statically known.

```go
strings.NewReplacer("old", "new")           // accepted
strings.NewReplacer("old", "new", "extra") // reported
```
