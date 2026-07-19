---
title: non-canonical-header
description: Detect non-canonical constant keys in http.Header reads.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Direct map reads do not canonicalize header keys. Header method calls and map
writes are not reported.

```go
value := header["content-type"] // reported
value := header["Content-Type"] // accepted
```
