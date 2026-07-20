---
title: task-comment
description: Surface TODO, FIXME, and BUG comments.
sidebar:
  badge:
    text: note
    class: severity-indicator severity-note
---

**Default severity:** <span class="severity-indicator severity-note" aria-hidden="true"></span> `note`

Reports source comments containing a `TODO`, `FIXME`, or `BUG` marker. Resolve
the task or link it to an owned work item before enforcing this advisory check.

## Bad

```go
// TODO: decide which errors should be retried.
```

## Good

```go
// Retry only errors classified as transient.
```
