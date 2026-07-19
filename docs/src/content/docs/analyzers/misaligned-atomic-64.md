---
title: misaligned-atomic-64
description: Detect misaligned 64-bit atomic field access on 32-bit targets.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

On 32-bit ARM, x86, and MIPS targets, callers must ensure that 64-bit words
passed to legacy `sync/atomic` functions are aligned to 8 bytes. Put the
64-bit atomic field first in its allocated struct, or use the typed atomic
wrappers that arrange their own alignment.

This check is target-aware and remains silent when loading a 64-bit build.

## Bad

```go
type counters struct {
    ready uint32
    total uint64 // may be misaligned on a 32-bit target
}
```

## Good

```go
type counters struct {
	total uint64
	ready uint32
}
```
