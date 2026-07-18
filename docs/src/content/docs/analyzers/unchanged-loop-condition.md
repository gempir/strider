---
title: unchanged-loop-condition
description: Detect counted loops whose condition variable never changes.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

A conventional three-part loop that initializes and tests one variable but
never changes that variable cannot progress as intended. This often means the
post statement increments the wrong counter or is unreachable.

```go
for index := 0; index < limit; other++ { // reported
    use(index)
}

for index := 0; index < limit; index++ { // accepted
    use(index)
}
```
