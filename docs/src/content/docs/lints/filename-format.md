---
title: filename-format
description: Allow only ASCII letters, digits, underscores, and hyphens in Go filenames.
sidebar:
  badge:
    text: note
    class: severity-indicator severity-note
---

**Default severity:** <span class="severity-indicator severity-note" aria-hidden="true"></span> `note`

Reports `.go` filenames containing characters outside ASCII letters, digits,
underscores, and hyphens. The filename must start with one of those characters
and end in `.go`; dots within the stem, spaces, and other punctuation are
rejected. It does not enforce snake_case or lowercase names.

## Bad

```go
// user.service.go
```

## Good

```go
// user_service.go
```
