---
title: interface-return
description: Detect constructors that hide a single concrete result.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Reports only exported `New`-style constructors whose returns consistently
reveal one local concrete implementation of a non-empty local interface.
Polymorphic, standard-library, empty, and error interfaces are skipped.
