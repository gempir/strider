---
title: excessive-blank-identifiers
description: Detect assignments that discard too many results.
---

**Default severity:** 🔵 `note`

Reports assignments containing three or more blank identifiers. Repeatedly
discarding adjacent results hides a function's contract and can conceal an
important value.

```go
value, _, _, _, err := load() // reported
value, metadata, err := load() // accepted
```
