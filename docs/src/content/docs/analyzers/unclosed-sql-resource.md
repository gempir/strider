---
title: unclosed-sql-resource
description: Detect locally acquired sql.Rows and sql.Stmt values that are not closed.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

Reports local `*sql.Rows` and `*sql.Stmt` acquisitions without a later close or
obvious ownership transfer. Check acquisition errors before deferring `Close`.
