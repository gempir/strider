---
title: test-parallelism
description: Identify tests and direct subtests that can run in parallel.
sidebar:
  badge:
    text: note
    class: severity-indicator severity-note
---

**Default severity:** <span class="severity-indicator severity-note" aria-hidden="true"></span> `note`

The advisory check suggests `t.Parallel()` for eligible top-level tests and
direct subtests. It skips tests that already opt in or visibly change process
state through environment, working-directory, or package-variable mutations.

## Bad

```go
func TestLoad(t *testing.T) {
	checkLoad(t)
}
```

## Good

```go
func TestLoad(t *testing.T) {
	t.Parallel()
	checkLoad(t)
}
```
