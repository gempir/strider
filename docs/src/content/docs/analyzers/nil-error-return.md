---
title: nil-error-return
description: Detect nil errors returned from branches that prove an error is non-nil.
---

**Default severity:** `error`

Reports explicit nil error results inside a branch entered because an error is
non-nil. Returning success there silently discards the failure.

```go
if err != nil { return nil, nil } // reported
if err != nil { return nil, err } // accepted
```
