---
title: untrappable-signal
description: Detect attempts to handle signals that cannot be trapped.
---

**Default severity:** 🟡 `warning`

Unix-like kernels handle `SIGKILL` and `SIGSTOP` directly. They are never
delivered to the process, so registering them with `os/signal` cannot work.

```go
signal.Notify(ch, os.Kill)          // reported
signal.Notify(ch, syscall.SIGTERM)  // accepted
```
