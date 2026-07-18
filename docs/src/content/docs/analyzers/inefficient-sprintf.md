---
title: inefficient-sprintf
description: Detect fmt.Sprintf calls used only for simple conversions.
---

**Default severity:** 🟡 `warning`

Simple `%s`, `%t`, and base-10 integer conversions can use the string directly
or the corresponding `strconv` function without format parsing and reflection.
