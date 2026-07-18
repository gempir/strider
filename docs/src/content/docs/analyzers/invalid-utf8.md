---
title: invalid-utf8
description: Detect invalid UTF-8 arguments to strings functions.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

Reports invalid constant cutsets and character lists passed to selected
`strings` functions.

```go
strings.Trim(value, "\xff") // reported
```
