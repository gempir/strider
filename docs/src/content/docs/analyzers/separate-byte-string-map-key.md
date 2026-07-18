---
title: separate-byte-string-map-key
description: Detect allocated byte-to-string temporaries used only for map lookups.
---

**Default severity:** 🟡 `warning`

The compiler can perform `items[string(bytes)]` without copying the byte slice
because the temporary string cannot escape the lookup. Assigning
`string(bytes)` to a variable first prevents that optimization and allocates
when the variable is used only as a map key.

The check stays silent when the string variable has any non-lookup use.

```go
key := string(bytes)
value := items[key] // reported at the conversion

value := items[string(bytes)] // accepted and allocation-free
```
