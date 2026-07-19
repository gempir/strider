---
title: waitgroup-go-forbidden-call
description: Reject panic, recover, and WaitGroup.Done inside WaitGroup.Go.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

`sync.WaitGroup.Go` calls `Done` automatically when its task returns, and its
contract requires the task not to panic. Calling `Done` manually corrupts the
counter; `panic` and `recover` indicate a task that does not satisfy the API
contract.

## Bad

```go
group.Go(func() {
	defer group.Done()
	work()
})
```

## Good

```go
group.Go(func() {
	work()
})
```
