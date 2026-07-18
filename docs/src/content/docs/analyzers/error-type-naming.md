---
title: error-type-naming
description: Name error implementations with an Error suffix.
sidebar:
  badge:
    text: note
    class: severity-indicator severity-note
---

**Default severity:** <span class="severity-indicator severity-note" aria-hidden="true"></span> `note`

Named types whose value or pointer method set implements `error` should end in
`Error`, making their role recognizable at API boundaries and in type
assertions.
