---
title: invalid-utf8
description: Detect invalid UTF-8 arguments to strings functions.
---

**Default severity:** 🔴 `error`

Reports invalid constant cutsets and character lists passed to selected
`strings` functions.

```go
strings.Trim(value, "\xff") // reported
```
