---
title: single-iteration-loop
description: Detect loops that always exit during their first iteration.
---

**Default severity:** 🟡 `warning`

A loop whose top-level control flow always returns or breaks after a
conditional branch cannot begin a second iteration. This usually means the
exit was placed outside the intended branch.

Map ranges and one-statement loops are accepted because selecting a single
arbitrary or first element is a common intentional pattern.

```go
for _, value := range values {
    if done(value) {
        use(value)
    }
    return // reported
}

for _, value := range values {
    if done(value) {
        break
    }
    use(value) // accepted
}
```
