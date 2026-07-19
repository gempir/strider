---
title: range-value-address
description: "Avoid taking addresses of range values."
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Taking the address of a range value points to the iteration copy, not the
corresponding slice or array element. Go 1.22 made variables declared with `:=`
iteration-local, so this is no longer the classic shared-pointer bug, but the
pointer still does not refer back to the source collection. Use an index when
that source identity is intended.

## Bad

```go
for _, value := range values {
	pointers = append(pointers, &value)
}
```

## Good

```go
for index := range values {
	pointers = append(pointers, &values[index])
}
```
