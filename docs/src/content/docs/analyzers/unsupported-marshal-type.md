---
title: unsupported-marshal-type
description: Detect channels and functions passed to JSON or XML marshaling.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

The standard JSON and XML encoders cannot marshal channel or function values.
The check recursively checks exported fields, while honoring ignored
fields and custom marshaling methods.

## Bad

```go
json.Marshal(make(chan int))
```

## Good

```go
json.Marshal(struct{ Name string }{Name: name})
```
