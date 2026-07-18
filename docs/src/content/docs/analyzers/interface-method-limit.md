---
title: interface-method-limit
description: Detect interfaces with more than ten methods.
sidebar:
  badge:
    text: note
    class: severity-indicator severity-note
---

**Default severity:** <span class="severity-indicator severity-note" aria-hidden="true"></span> `note`

Counts explicit and embedded methods after completing the interface. More than
ten suggests an abstraction that may be easier to implement and test when
split by responsibility.
