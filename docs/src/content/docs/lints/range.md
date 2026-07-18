---
title: range
description: Simplify range statements and avoid unnecessary rune slices.
---

Purpose: simplify range statements and remove allocations that do not change
the yielded values.

**Default severity:** `note`

## Behavior

The rule omits explicit blank range values and reports `range []rune(text)`
when the index is discarded. Ranging directly over the string yields the same
runes without allocating a slice. When the index is used, the conversion is
preserved because a string range yields byte offsets while a rune-slice range
yields rune indexes.

## Enable

This optional check runs when selected with `--only range`, enabled in
`strider.toml`, or included with `--all`.
