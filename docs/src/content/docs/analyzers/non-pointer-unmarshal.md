---
title: non-pointer-unmarshal
description: Detect non-pointer decoding and unmarshalling destinations.
---

**Default severity:** `error`

JSON and XML decoding APIs require a pointer destination so they can populate
the supplied value.

```go
json.Unmarshal(data, value) // reported
json.Unmarshal(data, &value) // accepted
```
