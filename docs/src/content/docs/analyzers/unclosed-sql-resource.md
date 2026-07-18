---
title: unclosed-sql-resource
description: Detect locally acquired sql.Rows and sql.Stmt values that are not closed.
---

**Default severity:** 🔴 `error`

Reports local `*sql.Rows` and `*sql.Stmt` acquisitions without a later close or
obvious ownership transfer. Check acquisition errors before deferring `Close`.
