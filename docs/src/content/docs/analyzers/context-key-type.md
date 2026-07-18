---
title: context-key-type
description: Detect unsafe context.WithValue key types.
---

**Default severity:** 🟡 `warning`

Context keys must be comparable and should use a dedicated named type to avoid
collisions between packages. Built-in types and anonymous empty structs risk
collisions; non-comparable and nil keys panic at runtime.

This type-aware check supersedes the earlier syntax-only implementation.

```go
context.WithValue(ctx, "request-id", value) // reported

type contextKey struct{}
context.WithValue(ctx, contextKey{}, value)  // accepted
```
