---
title: test-main-missing-exit
description: Detect legacy TestMain functions that lose the test exit code.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Before Go 1.15, a custom `TestMain` that called `testing.M.Run` had to pass its
result to `os.Exit` or failed tests could appear successful. Go 1.15 and newer
propagate the returned status automatically, so the check is silent there.

## Bad

```go
func TestMain(m *testing.M) { m.Run() }
```

## Good

```go
func TestMain(m *testing.M) { os.Exit(m.Run()) }
```
