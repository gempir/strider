---
title: redundant-conversion
description: Detect conversions to the value's existing type.
sidebar:
  badge:
    text: note
    class: severity-indicator severity-note
---

**Default severity:** <span class="severity-indicator severity-note" aria-hidden="true"></span> `note`

An exact same-type conversion cannot change a value or its method set. Remove
it to make the type flow clearer.
