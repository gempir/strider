---
title: excessive-blank-identifiers
description: Detect assignments that discard too many results.
sidebar:
  badge:
    text: note
    class: severity-indicator severity-note
---

**Default severity:** <span class="severity-indicator severity-note" aria-hidden="true"></span> `note`

Reports assignments containing three or more blank identifiers. Repeatedly
discarding adjacent results hides a function's contract and can conceal an
important value.

```go
value, _, _, _, err := load() // reported
value, metadata, err := load() // accepted
```
