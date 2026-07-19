---
title: separate-byte-string-map-key
description: Detect allocated byte-to-string temporaries used only for map lookups.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

The compiler can perform `items[string(bytes)]` without copying the byte slice
because the temporary string cannot escape the lookup. Assigning
`string(bytes)` to a variable first prevents that optimization and allocates
when the variable is used only as a map key.

The check stays silent when the string variable has any non-lookup use.

## Bad

```go
key := string(keyBytes); value := items[key]
```

## Good

```go
value := items[string(keyBytes)]
```
