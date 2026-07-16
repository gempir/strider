---
title: decimal-file-mode
description: Detect decimal file modes that look like octal permissions.
---

**Default severity:** `warning`

Unix permission modes are conventionally written in octal. A three-digit
decimal literal such as `644` passed as `os.FileMode` evaluates to a different
bit pattern than `0o644` and is usually a missing octal prefix.

```go
os.WriteFile(path, data, 644)   // reported
os.WriteFile(path, data, 0o644) // accepted
```
