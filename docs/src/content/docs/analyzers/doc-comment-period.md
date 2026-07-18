---
title: doc-comment-period
description: Require declaration documentation to end with punctuation.
sidebar:
  badge:
    text: note
    class: severity-indicator severity-note
---

**Default severity:** <span class="severity-indicator severity-note" aria-hidden="true"></span> `note`

Package and exported-declaration documentation should read as complete prose.
The final sentence may end with `.`, `!`, `?`, or `:`.

```go
// Client sends requests // reported
// Client sends requests. // accepted
```
