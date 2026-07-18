---
title: invalid-regexp
description: Detect invalid constant regular expressions.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

Reports invalid constant patterns passed to regular-expression compilation and
matching functions. Dynamic patterns are checked at runtime by the standard
library and are not reported.

```go
regexp.MustCompile(`[a-z`) // reported
regexp.MustCompile(`[a-z]+`) // accepted
```
