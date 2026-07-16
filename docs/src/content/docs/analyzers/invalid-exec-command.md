---
title: invalid-exec-command
description: Detect shell commands used as exec.Command program names.
---

**Default severity:** `warning`

`exec.Command` expects one executable name or path, not a shell command that
needs argument splitting.

```go
exec.Command("go test") // reported
exec.Command("go", "test") // accepted
```
