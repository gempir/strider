---
title: url-query-copy-mutation
description: Detect mutations of the temporary copy returned by URL.Query.
---

**Default severity:** 🟡 `warning`

`URL.Query` parses `RawQuery` and returns a new `Values` map. Mutating that
temporary map does not update the URL unless the encoded result is assigned
back to `RawQuery`.

```go
address.Query().Set(key, value) // reported

values := address.Query()
values.Set(key, value)
address.RawQuery = values.Encode() // accepted
```
