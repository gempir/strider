---
title: regexp-find-all-zero
description: Detect regexp FindAll calls with a zero result limit.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

A zero limit always returns no matches. Use a negative limit for all matches.

```go
expression.FindAllString(input, 0) // reported
expression.FindAllString(input, -1) // accepted
```
