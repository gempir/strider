---
title: cognitive-complexity
description: "Limit nested control-flow complexity."
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Reports a function when its cognitive-complexity score exceeds 7. Each control
structure adds one plus its nesting depth, so deeply nested decisions cost more
than an equivalent sequence of guard clauses.

## Bad

```go
func process(items []Item) {
	if ready() {
		for _, item := range items {
			if valid(item) {
				for retry(item) {
					process(item)
				}
			}
		}
	}
}
```

## Good

```go
func process(items []Item) {
	if !ready() {
		return
	}
	for _, item := range items {
		processItem(item)
	}
}
```
