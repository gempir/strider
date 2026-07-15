---
title: no-defer-in-loop
description: Avoid accumulating deferred calls across loop iterations.
---

**Default severity:** `warning`  
**Configuration:** no options

Reports `defer` statements nested inside `for` or `range` loops. Deferred calls
run when the surrounding function returns, not when the current iteration
ends, so resources can accumulate for the entire loop.

## Bad

```go
for _, filename := range filenames {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
}
```

## Good

```go
for _, filename := range filenames {
	if err := processFile(filename); err != nil {
		return err
	}
}

func processFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	return consume(file)
}
```

A defer inside a function literal declared within the loop is not reported:
that defer belongs to the nested function and runs when that invocation ends.

## Suppress

```go
//strider:ignore no-defer-in-loop
for range smallFixedSet {
	defer releaseAtFunctionExit()
}
```
