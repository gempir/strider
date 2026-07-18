---
title: ip-byte-comparison
description: Detect bytes.Equal comparisons between IP addresses.
---

**Default severity:** 🟡 `warning`

An IPv4 address stored in `net.IP` may have either a 4-byte or 16-byte
representation. `bytes.Equal` treats those representations as different;
`net.IP.Equal` compares the address values correctly.

```go
bytes.Equal(left, right) // reported when both values are net.IP
left.Equal(right)        // accepted
```
