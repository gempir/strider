---
title: error-type-naming
description: Name error implementations with an Error suffix.
---

**Default severity:** 🔵 `note`

Named types whose value or pointer method set implements `error` should end in
`Error`, making their role recognizable at API boundaries and in type
assertions.
