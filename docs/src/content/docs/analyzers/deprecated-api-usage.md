---
title: deprecated-api-usage
description: Detect uses of deprecated packages and APIs.
---

**Default severity:** `warning`

Go documentation can mark packages, functions, methods, fields, variables,
constants, and types with a `Deprecated:` paragraph. This check reads those
markers from loaded dependencies and reports resolved uses from other
packages, including deprecated struct fields.

Standard-library deprecations are reported only when the module targets the
running Go language version or newer, avoiding recommendations based on API
documentation newer than an older module's target.

```go
ioutil.ReadAll(reader) // reported
io.ReadAll(reader)     // accepted
```
