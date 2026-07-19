---
title: add-constant
description: "Suggest named constants for repeated literals."
sidebar:
  badge:
    text: note
    class: severity-indicator severity-note
---

**Default severity:** <span class="severity-indicator severity-note" aria-hidden="true"></span> `note`

Reports a string literal when it appears for the third time in one file. A
named constant keeps repeated protocol values, states, and keys consistent.

## Bad

```go
if state == "ready" { start() }
if next == "ready" { queue() }
if previous == "ready" { resume() }
```

## Good

```go
const stateReady = "ready"
if state == stateReady { start() }
```
