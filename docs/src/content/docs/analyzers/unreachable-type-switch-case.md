---
title: unreachable-type-switch-case
description: Detect type-switch cases hidden by earlier interfaces.
---

**Default severity:** `warning`

Type-switch cases are evaluated in source order. A later concrete type or
narrower interface is unreachable when it necessarily implements an interface
listed by an earlier case.

```go
switch value.(type) {
case io.Reader:
case io.ReadCloser: // reported: every ReadCloser is already a Reader
}

switch value.(type) {
case io.ReadCloser:
case io.Reader: // accepted: not every Reader is a ReadCloser
}
```
