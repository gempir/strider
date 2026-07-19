---
title: regexp-match-in-loop
description: Detect repeated regexp compilation inside loops.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

The package-level regexp matching helpers compile their pattern on every call.
Calling them with a constant pattern inside a loop repeats the same compilation.
Compile the expression once before the loop and reuse it.

Dynamic patterns are accepted because hoisting them may change behavior.

## Bad

```go
for _, value := range values { regexp.MatchString(`^[a-z]+$`, value) }
```

## Good

```go
pattern := regexp.MustCompile(`^[a-z]+$`); for _, value := range values { pattern.MatchString(value) }
```
