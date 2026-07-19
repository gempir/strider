---
title: interface-method-limit
description: Detect interfaces with more than ten methods.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Counts explicit and embedded methods after completing the interface. More than
ten suggests an abstraction that may be easier to implement and test when
split by responsibility.

## Configuration

The default maximum is ten methods, including embedded methods.

```toml
[checks.rules.interface-method-limit]
max-methods = 12
```
