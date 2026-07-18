---
title: slog-argument-shape
description: Detect malformed or inconsistent log/slog arguments.
---

**Default severity:** 🟡 `warning`

Reports odd key/value tails, non-string loose keys, and calls that mix
`slog.Attr` values with loose key/value pairs.
