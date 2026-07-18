---
title: inline-error-declaration
description: Detect error variables declared in control-statement initializers.
sidebar:
  badge:
    text: note
    class: severity-indicator severity-note
---

**Default severity:** <span class="severity-indicator severity-note" aria-hidden="true"></span> `note`

Error declarations in `if` and `switch` initializers have a narrow scope and
can make error-handling paths dense. Declare the value immediately before the
control statement when a longer-lived value is clearer.

```go
value, err := load()
if err != nil {
    return err
}
```
