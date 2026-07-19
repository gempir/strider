---
title: unchecked-rows-error
description: Detect sql.Rows iteration without an Err check.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

`Rows.Next` returns false both when iteration finishes successfully and when an
iteration error occurs. Check `Rows.Err` after the loop so driver, network, and
decoding failures are not silently treated as successful completion.

## Bad

```go
for rows.Next() { scan(rows) }
```

## Good

```go
for rows.Next() { scan(rows) }; if err := rows.Err(); err != nil { return err }
```
