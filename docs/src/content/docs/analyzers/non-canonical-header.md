---
title: non-canonical-header
description: Detect non-canonical constant keys in http.Header reads.
---

**Default severity:** `note`

Direct map reads do not canonicalize header keys. Header method calls and map
writes are not reported.

```go
value := header["content-type"] // reported
value := header["Content-Type"] // accepted
```
