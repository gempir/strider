---
title: invalid-template
description: Detect invalid constant text and HTML templates.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

Checks constant templates in direct `template.New(...).Parse(...)` chains.
Receivers that may use custom delimiters are left alone.

```go
template.New("greeting").Parse(`Hello, {{.Name}`) // reported
```
