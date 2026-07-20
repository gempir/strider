---
title: excessive-blank-identifiers
description: Detect assignments that discard too many results.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Reports assignments containing three or more blank identifiers. Repeatedly
discarding adjacent results hides a function's contract and can conceal an
important value.

## Bad

```go
value, _, _, _, err := load()
```

## Good

```go
value, metadata, err := load(); _ = metadata
```
