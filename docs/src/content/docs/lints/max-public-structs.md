---
title: max-public-structs
description: "Limit exported structs per file."
sidebar:
  badge:
    text: note
    class: severity-indicator severity-note
---

**Default severity:** <span class="severity-indicator severity-note" aria-hidden="true"></span> `note`

Limit exported structs per file.

## Bad

```go
type Request struct{}
type Response struct{}
type Client struct{}
type Server struct{}
type Options struct{}
type Result struct{}
```

## Good

```go
type Request struct{}
type Response struct{}
```

## Configuration

| Option | Type | Default | Description |
| --- | --- | --- | --- |
| `max-public-structs` | `int` | `5` | Maximum number of exported struct declarations allowed per file. |

```toml
[checks.max-public-structs]
max-public-structs = 5
```
