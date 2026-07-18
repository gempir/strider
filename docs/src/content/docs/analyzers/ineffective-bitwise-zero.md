---
title: ineffective-bitwise-zero
description: Detect bitwise operations whose zero operand fixes the result.
---

**Default severity:** 🟡 `warning`

For integers, `x & 0` is always zero while `x | 0` and `x ^ 0` are always
`x`. A zero-valued flag declared directly with `iota` often indicates that
`1 << iota` was intended.

Shift-by-zero expressions are accepted because they commonly appear in regular
bit-field layouts alongside shifts by 8, 16, and so on.

```go
unchanged := value ^ 0 // reported
masked := value & mask // accepted
shifted := value << 0  // accepted
```
