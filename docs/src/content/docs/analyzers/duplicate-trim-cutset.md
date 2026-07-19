---
title: duplicate-trim-cutset
description: Detect duplicate characters in string trim cutsets.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

`strings.Trim`, `TrimLeft`, and `TrimRight` interpret their second argument as
a set of runes, not as a prefix or suffix. Duplicate runes have no effect and
often reveal that `TrimPrefix` or `TrimSuffix` was intended.

## Bad

```go
strings.TrimLeft(value, "letter")
```

## Good

```go
strings.TrimPrefix(value, "prefix")
```
