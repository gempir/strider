---
title: context-stored-in-struct
description: Detect context.Context fields in structs.
---

**Default severity:** 🟡 `warning`

Contexts carry request-scoped cancellation, deadlines, and values. Keeping one
in a struct obscures its lifetime and can accidentally reuse stale request
state. Pass the context explicitly to each operation that needs it.

```go
type Service struct {
	ctx context.Context // reported
}

func (service *Service) Run(ctx context.Context) error { // accepted
	return service.run(ctx)
}
```
