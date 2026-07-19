---
title: redundant-conversion
description: Detect conversions to the value's existing type.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

An exact same-type conversion cannot change a value or its method set. Remove
it to make the type flow clearer.

## Bad

```go
normalized := UserID(existingUserID)
```

## Good

```go
normalized := UserID(rawID)
```
