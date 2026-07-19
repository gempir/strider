---
title: spinning-select-default
description: Detect select loops that spin on an empty default.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

An empty `default` makes a `select` immediately ready. Inside an unconditional
loop, this prevents the goroutine from blocking and consumes CPU continuously.
Remove the empty default so the select can wait for a communication case.

## Bad

```go
for { select { case value := <-values: use(value); default: } }
```

## Good

```go
for { select { case value := <-values: use(value) } }
```
