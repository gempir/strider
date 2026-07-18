---
title: inefficient-sprintf
description: Detect fmt.Sprintf calls used only for simple conversions.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Simple `%s`, `%t`, and base-10 integer conversions can use the string directly
or the corresponding `strconv` function without format parsing and reflection.
