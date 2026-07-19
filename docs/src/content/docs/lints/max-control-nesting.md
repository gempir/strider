---
title: max-control-nesting
description: "Limit nested control structures."
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Reports control flow nested more than five levels deep. Guard clauses and
cohesive helper functions usually make the same decisions easier to follow.

## Bad

```go
if first {
	if second {
		if third {
			if fourth {
				if fifth {
					if sixth {
						process()
					}
				}
			}
		}
	}
}
```

## Good

```go
if !ready {
	return
}
for _, item := range items {
	process(item)
}
```
