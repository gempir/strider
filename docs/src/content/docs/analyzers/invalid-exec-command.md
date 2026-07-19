---
title: invalid-exec-command
description: Detect shell commands used as exec.Command program names.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

`exec.Command` expects one executable name or path, not a shell command that
needs argument splitting.

## Bad

```go
exec.Command("ls / /tmp")
```

## Good

```go
exec.Command("ls", "/", "/tmp")
```
