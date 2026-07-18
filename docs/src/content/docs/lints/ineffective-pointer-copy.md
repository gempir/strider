---
title: ineffective-pointer-copy
description: Detect pointer round trips that do not copy values.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Go simplifies `&*pointer` to `pointer` and `*&value` to `value`. Neither form
copies the underlying data, so code using one as a copy operation is
misleading and usually incorrect.

```go
copy := &*pointer // reported: still the same pointer
copy := *pointer  // an actual value copy
```
