---
title: unexported-serialization-fields
description: Detect serialization of structs with no exported fields.
---

**Default severity:** 🟡 `warning`

The standard JSON and XML packages ignore unexported struct fields. Marshaling
a non-empty struct with no exported fields produces empty data, and
unmarshaling into it cannot populate anything.

Empty structs, promoted exported fields, and types with custom text, JSON, or
XML serialization methods are accepted.

```go
json.Marshal(struct{ value string }{"hidden"}) // reported
json.Marshal(struct{ Value string }{"visible"}) // accepted
```
