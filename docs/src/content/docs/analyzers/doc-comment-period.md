---
title: doc-comment-period
description: Require declaration documentation to end with punctuation.
---

**Default severity:** 🔵 `note`

Package and exported-declaration documentation should read as complete prose.
The final sentence may end with `.`, `!`, `?`, or `:`.

```go
// Client sends requests // reported
// Client sends requests. // accepted
```
