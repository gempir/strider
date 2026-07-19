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

The default maximum is five exported structs per file.

```toml
[checks.rules.max-public-structs]
max-public-structs = 8
```

Set `max-public-structs = 0` to use the built-in maximum of five.
