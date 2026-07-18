---
title: nil-value-with-nil-error
description: Detect nil payloads returned together with a nil error.
---

**Default severity:** `warning`

For functions returning a nilable payload and a final error, an explicit
`nil, nil` result makes absence indistinguishable from successful production of
a value. Documented APIs that intentionally use this contract can exclude it.
