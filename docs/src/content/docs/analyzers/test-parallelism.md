---
title: test-parallelism
description: Identify tests and direct subtests that can run in parallel.
---

**Default severity:** `note`

The advisory check suggests `t.Parallel()` for eligible top-level tests and
direct subtests. It skips tests that already opt in or visibly change process
state through environment, working-directory, or package-variable mutations.

```go
func TestLoad(t *testing.T) {
    t.Parallel()
    checkLoad(t)
}
```
