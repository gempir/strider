---
title: file-length-limit
description: "Limit source-file length."
sidebar:
  badge:
    text: note
    class: severity-indicator severity-note
---

**Default severity:** <span class="severity-indicator severity-note" aria-hidden="true"></span> `note`

Limits source-file length. The default limit is `0`, which disables the check.

## Configuration

```toml
[checks.rules.file-length-limit]
max-lines = 500
```
