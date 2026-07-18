---
title: invalid-regexp
description: Detect invalid constant regular expressions.
---

**Default severity:** 🔴 `error`

Reports invalid constant patterns passed to regular-expression compilation and
matching functions. Dynamic patterns are checked at runtime by the standard
library and are not reported.

```go
regexp.MustCompile(`[a-z`) // reported
regexp.MustCompile(`[a-z]+`) // accepted
```
