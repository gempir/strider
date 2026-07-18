---
title: standard-http-method-constant
description: Prefer net/http method constants.
---

**Default severity:** 🔵 `note`

Reports hardcoded standard methods passed to `http.NewRequest` and
`http.NewRequestWithContext`. Use constants such as `http.MethodGet` to make
protocol intent explicit.
