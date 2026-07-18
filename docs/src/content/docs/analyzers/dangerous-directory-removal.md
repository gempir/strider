---
title: dangerous-directory-removal
description: Detect removal of whole system or user directories.
---

**Default severity:** `error`

Passing the direct result of `os.TempDir` or a user directory helper to
`os.RemoveAll` deletes the entire shared directory rather than an
application-owned child. This is commonly caused by confusing `TempDir` with a
directory-creation helper or forgetting to append a suffix.

The check covers the temporary, user cache, user configuration, and user
home directory helpers.

```go
directory := os.TempDir()
os.RemoveAll(directory) // reported

directory := filepath.Join(os.TempDir(), "application")
os.RemoveAll(directory) // accepted
```
