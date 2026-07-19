---
title: standard-http-method-constant
description: Prefer net/http method constants.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Reports hardcoded standard methods passed to `http.NewRequest` and
`http.NewRequestWithContext`. Use constants such as `http.MethodGet` to make
protocol intent explicit.

## Bad

```go
request, err := http.NewRequest("GET", endpoint, nil)
```

## Good

```go
request, err := http.NewRequest(http.MethodGet, endpoint, nil)
```
