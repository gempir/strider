---
title: discarded-pure-result
description: Detect ignored results from functions without side effects.
---

**Default severity:** 🟡 `warning`

Calling a function that has no side effects and then discarding all of its
return values cannot affect program behavior. The call is either dead code or
its result was meant to be used.

Purity is inferred conservatively from SSA. Calls involving external memory,
channels, goroutines, panic, escaping allocations, or unknown callees are not
reported. Benchmark helpers accepting `*testing.B` are exempt because invoking
otherwise pure work is a common measurement pattern.

```go
strings.TrimSpace(input)            // reported
cleaned := strings.TrimSpace(input) // accepted
```
