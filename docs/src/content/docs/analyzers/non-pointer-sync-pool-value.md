---
title: non-pointer-sync-pool-value
description: Detect sync.Pool values that allocate while being stored.
---

**Default severity:** 🟡 `warning`

`sync.Pool.Put` accepts an interface. Storing a concrete non-pointer value
requires boxing it on the heap, adding the allocation the pool is intended to
avoid. Slices are also boxed because the slice header itself is a value.

Store a pointer to the reusable value instead. Pointer-like maps, channels,
functions, interfaces, and unsafe pointers are accepted.

```go
pool.Put(buffer)  // reported when buffer is []byte
pool.Put(&buffer) // accepted
```
