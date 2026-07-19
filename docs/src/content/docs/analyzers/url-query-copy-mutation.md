---
title: url-query-copy-mutation
description: Detect mutations of the temporary copy returned by URL.Query.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

`URL.Query` parses `RawQuery` and returns a new `Values` map. Mutating that
temporary map does not update the URL unless the encoded result is assigned
back to `RawQuery`.

## Bad

```go
address.Query().Set(key, value)
```

## Good

```go
values := address.Query(); values.Set(key, value); address.RawQuery = values.Encode()
```
