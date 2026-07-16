---
title: invalid-template
description: Detect invalid constant text and HTML templates.
---

**Default severity:** `error`

Checks constant templates in direct `template.New(...).Parse(...)` chains.
Receivers that may use custom delimiters are left alone.

```go
template.New("greeting").Parse(`Hello, {{.Name}`) // reported
```
