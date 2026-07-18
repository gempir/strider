---
title: invalid-printf-call
description: Detect malformed printf formats and mismatched arguments.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

Printf-style calls interpret a small language of verbs, argument indexes,
widths, and precisions. This check validates constant formats against the
resolved call and its variadic arguments.

It reports malformed formats, unknown verbs, missing or extra arguments,
non-integer dynamic width or precision arguments, unsupported `%w` wrapping,
and incompatible value types. Calls with a spread variadic slice are accepted
when their individual values cannot be determined statically.

```go
fmt.Printf("%d %s", count, name) // accepted
fmt.Printf("%d %s", name, count) // reported
```
