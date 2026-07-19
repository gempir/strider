---
title: simplify-range
description: Simplify range statements and avoid unnecessary rune slices.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

Simplify range statements and remove allocations that do not change
the yielded values.

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

## Behavior

The rule omits explicit blank range values and reports `range []rune(text)`
when the index is discarded. Ranging directly over the string yields the same
runes without allocating a slice. When the index is used, the conversion is
preserved because a string range yields byte offsets while a rune-slice range
yields rune indexes.

## Bad

```go
for _, character := range []rune(text) {
	use(character)
}

for _, _ = range values {
	use(values)
}
```

## Good

```go
for _, character := range text {
	use(character)
}

for range values {
	use(values)
}
```
