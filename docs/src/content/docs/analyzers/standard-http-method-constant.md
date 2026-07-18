---
title: standard-http-method-constant
description: Prefer net/http method constants.
sidebar:
  badge:
    text: note
    class: severity-indicator severity-note
---

**Default severity:** <span class="severity-indicator severity-note" aria-hidden="true"></span> `note`

Reports hardcoded standard methods passed to `http.NewRequest` and
`http.NewRequestWithContext`. Use constants such as `http.MethodGet` to make
protocol intent explicit.
