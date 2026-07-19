---
title: copy-lock-value
description: Detect values that copy sync locks.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

Copying a value that contains `sync.Mutex` or `sync.RWMutex` creates an
independent lock state and can invalidate the intended synchronization. Pass
such values by pointer and avoid value receivers, assignments, and returns that
copy them.

## Bad

```go
func update(state State) {
	state.mu.Lock()
	defer state.mu.Unlock()
}
```

## Good

```go
func update(state *State) {
	state.mu.Lock()
	defer state.mu.Unlock()
}
```
