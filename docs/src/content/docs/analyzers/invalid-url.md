---
title: invalid-url
description: Detect invalid constant URLs passed to net/url.Parse.
---

**Default severity:** 🔴 `error`

```go
url.Parse(":") // reported
url.Parse("https://go.dev") // accepted
```
