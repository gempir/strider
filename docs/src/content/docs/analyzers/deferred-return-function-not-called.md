---
title: deferred-return-function-not-called
description: Detect deferred setup calls whose returned function is not called.
---

**Default severity:** `warning`

Functions sometimes perform setup immediately and return a function that
performs cleanup. Deferring only the first call delays setup until function
exit and then discards the returned cleanup function.

```go
defer setup()  // reported: setup itself is deferred; cleanup is discarded
defer setup()() // accepted: setup runs now and cleanup is deferred
```
