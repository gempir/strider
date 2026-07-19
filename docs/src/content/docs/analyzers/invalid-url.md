---
title: invalid-url
description: Detect invalid constant URLs passed to net/url.Parse.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

## Bad

```go
url.Parse(":")
```

## Good

```go
url.Parse("https://golang.org")
```
