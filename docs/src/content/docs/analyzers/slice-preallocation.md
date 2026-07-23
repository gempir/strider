---
title: slice-preallocation
description: Detect slices that can use range-source capacity.
sidebar:
  badge:
    text: note
    class: severity-indicator severity-note
---

**Default severity:** <span class="severity-indicator severity-note" aria-hidden="true"></span> `note`

Conservatively reports an empty slice followed by exactly one direct append per
iteration of a range with a useful `len`. Preallocating that capacity avoids
repeated growth while preserving zero length.

## Bad

```go
var result []Item
for _, item := range source {
	result = append(result, convert(item))
}
```

## Good

```go
result := make([]Item, 0, len(source))
for _, item := range source {
	result = append(result, convert(item))
}
```
