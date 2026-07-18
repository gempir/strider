---
title: context-cancel-in-loop
description: Detect derived contexts retained across loop iterations.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

`context.WithCancel`, `WithTimeout`, and `WithDeadline` retain parent or timer
resources until cancellation. A defer in the surrounding function runs too
late; cancel during the iteration or move the body into a helper.
