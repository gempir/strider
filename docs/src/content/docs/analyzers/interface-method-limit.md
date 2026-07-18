---
title: interface-method-limit
description: Detect interfaces with more than ten methods.
---

**Default severity:** 🔵 `note`

Counts explicit and embedded methods after completing the interface. More than
ten suggests an abstraction that may be easier to implement and test when
split by responsibility.
