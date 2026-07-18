---
title: ineffective-value-receiver-assignment
description: Detect field assignments that cannot escape a value receiver.
---

**Default severity:** 🟡 `warning`

A method with a value receiver modifies only its local receiver copy. When an
assigned field is never read afterward, the write has no observable effect and
often indicates that the method should use a pointer receiver.

The check follows field reads through the method's control-flow graph. It
does not report a local mutation that is subsequently read to compute a result.

```go
func (item Item) Rename(name string) {
    item.Name = name // reported
}

func (item *Item) Rename(name string) {
    item.Name = name // accepted
}
```
