---
title: error-type-naming
description: Name error implementations with an Error suffix.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Named types whose value or pointer method set implements `error` should end in
`Error`, making their role recognizable at API boundaries and in type
assertions.
