---
title: context-stored-in-struct
description: Detect context.Context fields in structs.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Contexts carry request-scoped cancellation, deadlines, and values. Keeping one
in a struct obscures its lifetime and can accidentally reuse stale request
state. Pass the context explicitly to each operation that needs it.

## Bad

```go
type Service struct { ctx context.Context }
```

## Good

```go
func (service *Service) Run(ctx context.Context) error
```
