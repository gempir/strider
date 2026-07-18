---
title: decimal-file-mode
description: Detect decimal file modes that look like octal permissions.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Unix permission modes are conventionally written in octal. A three-digit
decimal literal such as `644` passed as `os.FileMode` evaluates to a different
bit pattern than `0o644` and is usually a missing octal prefix.

```go
os.WriteFile(path, data, 644)   // reported
os.WriteFile(path, data, 0o644) // accepted
```
