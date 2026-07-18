---
title: invalid-url
description: Detect invalid constant URLs passed to net/url.Parse.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

```go
url.Parse(":") // reported
url.Parse("https://go.dev") // accepted
```
