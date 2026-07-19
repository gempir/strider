---
title: non-pointer-unmarshal
description: Detect non-pointer decoding and unmarshalling destinations.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

JSON and XML decoding APIs require a pointer destination so they can populate
the supplied value.

## Bad

```go
json.Unmarshal(data, value)
```

## Good

```go
json.Unmarshal(data, &value)
```
