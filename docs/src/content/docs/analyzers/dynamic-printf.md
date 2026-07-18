---
title: dynamic-printf
description: Detect printf-style calls with a lone dynamic format.
---

**Default severity:** 🟡 `warning`

Reports supported printf-style calls whose only format argument is dynamic.
Use a print-style function or an explicit `%s` format.

```go
fmt.Printf(message) // reported
fmt.Printf("%s", message) // accepted
```
