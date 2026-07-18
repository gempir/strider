---
title: invalid-listen-address
description: Detect invalid constant HTTP listen addresses.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

`http.ListenAndServe` and `http.ListenAndServeTLS` expect a host and port
separated by a colon. Either side may be omitted. Numeric ports must be in
range; service names must use their supported letter, digit, and hyphen form.

```go
http.ListenAndServe("localhost", handler) // reported
http.ListenAndServe(":8080", handler)     // accepted
```
