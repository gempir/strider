---
title: unexported-serialization-fields
description: Detect serialization of structs with no exported fields.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

The standard JSON and XML packages ignore unexported struct fields. Marshaling
a non-empty struct with no exported fields produces empty data, and
unmarshaling into it cannot populate anything.

Empty structs, promoted exported fields, and types with custom text, JSON, or
XML serialization methods are accepted.

## Bad

```go
json.Marshal(struct{ name string }{name: name})
```

## Good

```go
json.Marshal(struct{ Name string }{Name: name})
```
