---
title: file-length-limit
description: "Limit source-file length."
sidebar:
  badge:
    text: note
    class: severity-indicator severity-note
---

**Default severity:** <span class="severity-indicator severity-note" aria-hidden="true"></span> `note`

Limits source-file length. The built-in maximum is 500 lines.

For example, with `max-lines = 5`, a six-line source file is reported while a
five-line file is accepted.

## Bad

```go
package example

const first = 1
const second = 2
const third = 3
const fourth = 4
```

## Good

```go
package example

const first = 1
const second = 2
const third = 3
```

## Configuration

```toml
[checks.file-length-limit]
max-lines = 800
```

Set `max-lines = 0` to disable this check explicitly.
