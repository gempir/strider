---
title: impossible-platform-comparison
description: Detect GOOS and GOARCH comparisons excluded by build constraints.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

A file's build constraints limit the operating systems and architectures on
which its code can run. Comparing `runtime.GOOS` or `runtime.GOARCH` with an
excluded known target has a fixed result.

Platform aliases are respected: Android satisfies the `linux` tag, iOS
satisfies `darwin`, and Illumos satisfies `solaris`.

## Bad

```go
//go:build linux

if runtime.GOOS == "windows" { // reported
    unreachable()
}
```

## Good

```go
//go:build linux

if runtime.GOOS == "linux" {
	useLinuxPath()
}
```
