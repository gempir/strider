---
title: duplicate-trim-cutset
description: Detect duplicate characters in string trim cutsets.
---

**Default severity:** 🟡 `warning`

`strings.Trim`, `TrimLeft`, and `TrimRight` interpret their second argument as
a set of runes, not as a prefix or suffix. Duplicate runes have no effect and
often reveal that `TrimPrefix` or `TrimSuffix` was intended.

```go
strings.TrimLeft(value, "letter")   // reported: repeated runes
strings.TrimPrefix(value, "letter") // accepted
```
