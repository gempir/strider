---
title: bidirectional-control-character
description: Reject invisible bidirectional source controls.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

Purpose: detect invisible Unicode controls that can change the visual order of
source text without changing its logical byte order. Such controls can make
reviewed code appear to mean something different from what the compiler reads.

The rule covers embedding, override, isolate, and matching pop controls. Write
ordinary direction-neutral source text instead.

## Bad

The comment below contains an invisible `U+202E RIGHT-TO-LEFT OVERRIDE` before
`denied`, which can make the source appear in a different order:

```go
// access ‮denied
```

## Good

```go
// access denied
```
