---
title: testmain-missing-exit
description: Detect legacy TestMain functions that lose the test exit code.
---

**Default severity:** `warning`

Before Go 1.15, a custom `TestMain` that called `testing.M.Run` had to pass its
result to `os.Exit` or failed tests could appear successful. Go 1.15 and newer
propagate the returned status automatically, so the check is silent there.

```go
func TestMain(m *testing.M) { m.Run() }          // reported before Go 1.15
func TestMain(m *testing.M) { os.Exit(m.Run()) } // accepted
```
